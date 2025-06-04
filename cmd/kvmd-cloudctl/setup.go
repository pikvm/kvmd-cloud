package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tmaxmax/go-sse"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/pikvm/cloud-api/api_models"
	"github.com/pikvm/kvmd-cloud/internal/config"
)

const (
	authCheckUrl          = "https://localhost/api/auth/check"
	checkPasswordsWikiUrl = "https://docs.pikvm.org/first_steps/#getting-access-to-pikvm"
)

var (
	authFilepath  = "/etc/kvmd/cloud/auth.yaml"
	nginxFilepath = "/etc/kvmd/cloud/nginx.ctx-http.conf"
	envishere     = false // envishere is a dev marker
)

//go:embed configs/nginx.http.conf
var nginxHttpContent []byte

//go:embed configs/nginx.http-and-https.conf
var nginxHttpsContent []byte

func Setup(cmd *cobra.Command, args []string) error {
	if !envishere {
		if err := checkLocalAuth(); err != nil {
			return fmt.Errorf("local kvmd auth check failed: %w", err)
		}
	}

	token, err := obtainCreds(cmd.Context())
	if err != nil {
		return err
	}
	config.Cfg.AuthToken = token

	logrus.Info("Performing a cloud connection attempt...")
	me, err := whoami(cmd.Context(), token)
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}
	logrus.Infof("Authorization successful. My name: %s/%s", me.User.Name, me.Name)

	if err := saveAuthData(); err != nil {
		return fmt.Errorf("unable to save authorization data: %w", err)
	}
	logrus.Info("Authorization information saved")

	if envishere {
		return nil
	}

	var nginxAffected bool = false
	logrus.Info("Preparing http configuration for letsencrypt...")
	if err := os.WriteFile(nginxFilepath, nginxHttpContent, 0644); err != nil {
		return fmt.Errorf("unable to write nginx configuration: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "restart", "kvmd-nginx"}); err != nil {
		return fmt.Errorf("unable to restart nginx: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-cloud"}); err != nil {
		return fmt.Errorf("unable to start kvmd-cloud agent: %w", err)
	}
	logrus.Info("Requesting letsencrypt SSL certificate...")
	if err := launchCmd([]string{
		"kvmd-certbot", "certonly_webroot", "--agree-tos", "-n",
		"--email", me.User.Email,
		"-d", me.DefaultFqdn,
	}); err != nil {
		return fmt.Errorf("unable to get certificate: %w", err)
	}
	nginxAffected = true
	defer func() { restoreNginx(nginxAffected) }()
	if err := os.WriteFile(nginxFilepath, nginxHttpsContent, 0664); err != nil {
		return fmt.Errorf("unable to write nginx configuration: %w", err)
	}
	if err := launchCmd([]string{"kvmd-certbot", "install_cloud", me.DefaultFqdn}); err != nil {
		return fmt.Errorf("unable to install certificate: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-certbot.timer"}); err != nil {
		return fmt.Errorf("unable to install certificate: %w", err)
	}

	logrus.Info("Your system is accessible externally via https://" + me.DefaultFqdn)

	logrus.Info("Done. Please, ensure that you password is strong enough")

	nginxAffected = false
	return nil
}

func launchCmd(cmdParts []string) error {
	cmd := exec.Command(cmdParts[0], cmdParts[1:]...)
	out, err := cmd.CombinedOutput()
	if exitErr, isExitErr := err.(*exec.ExitError); isExitErr {
		exitErr.ProcessState.ExitCode()
		return fmt.Errorf("command exited with code %d. Output: %s", exitErr.ProcessState.ExitCode(), string(out))
	} else if err != nil {
		return fmt.Errorf("command execution error: %w", err)
	}
	return nil
}

func obtainCreds(ctx context.Context) (string, error) {
	token, err := browserAuth(ctx)
	if err != nil || token == "" {
		token, err = askCreds()
		if err != nil {
			return "", err
		}
	}
	return token, nil
}

func browserAuth(ctx context.Context) (string, error) {
	logrus.Info("Obtaining bootstrap URL")
	url, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agents/bootstrap")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var token string
	var done bool
	var eventError error

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return "", err
	}
	conn := sse.NewConnection(r)
	failedEvent := ""

	conn.SubscribeEvent("redirect", func(e sse.Event) {
		redirect := api_models.BootstrapRedirect{}
		resp := api_models.ResponseModel{Result: &redirect}
		err := json.Unmarshal([]byte(e.Data), &resp)
		if err == nil && resp.Error != nil {
			err = errors.New(resp.Error.Error())
		}
		if err != nil {
			eventError = err
			failedEvent = "redirect"
			cancel()
			return
		}
		logrus.WithField("payload", redirect).Debug("received redirect event")
		fmt.Printf("Please, open the following URL in your browser and follow instructions: %s\n", redirect.RedirectURL)
	})

	conn.SubscribeEvent("result", func(e sse.Event) {
		result := api_models.BootstrapResult{}
		resp := api_models.ResponseModel{Result: &result}
		err := json.Unmarshal([]byte(e.Data), &resp)
		if err == nil && resp.Error != nil {
			err = errors.New(resp.Error.Error())
		}
		if err != nil {
			eventError = err
			failedEvent = "result"
			cancel()
			return
		}
		logrus.WithField("payload", result).Debug("received result event")
		token = result.AuthToken
		done = true
		cancel()
	})

	err = conn.Connect()
	if errors.Is(err, context.Canceled) {
		err = nil
	}
	if eventError != nil {
		logrus.WithError(eventError).WithField("url", url).WithField("failed_event", failedEvent).Error("event error")
		return "", eventError
	} else if err != nil {
		logrus.WithError(err).WithField("url", url).Error("connection error")
	}

	if done {
		return token, nil
	}
	return token, err
}

func askCreds() (token string, err error) {
	fmt.Println("Input authorization data:")

	fmt.Print("Authorization token: ")
	var b []byte
	b, err = term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return
	}
	token = string(b)

	return
}

func whoami(ctx context.Context, token string) (*api_models.WhoamiResult, error) {
	httpc := &http.Client{
		Timeout: 5 * time.Second,
	}
	url, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agents/whoami")
	if err != nil {
		return nil, err
	}
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	resp, err := httpc.Do(r)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	me := &api_models.WhoamiResult{}
	response := api_models.ResponseModel{Result: me}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return nil, err
	}
	if response.Error != nil {
		return nil, errors.New(response.Error.Error())
	}

	return me, nil
}

func saveAuthData() error {
	authFileContent := struct {
		AuthToken string `yaml:"auth_token"`
	}{
		AuthToken: config.Cfg.AuthToken,
	}
	out, err := yaml.Marshal(&authFileContent)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(authFilepath), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(authFilepath, out, 0644); err != nil {
		return err
	}
	return nil
}

func checkLocalAuth() error {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := http.Client{
		Timeout:   1 * time.Second,
		Transport: tr,
	}
	req, err := http.NewRequest("GET", authCheckUrl, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		fmt.Printf("%#v", err)
		return err
	}
	res.Body.Close()
	if res.StatusCode == 200 {
		return fmt.Errorf("local authorization disabled. Please, enable authorization and set a strong password")
	} else if res.StatusCode != 401 {
		return fmt.Errorf("weird status code %d for %s with no password", res.StatusCode, authCheckUrl)
	}
	req, err = http.NewRequest("GET", authCheckUrl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-KVMD-User", "admin")
	req.Header.Set("X-KVMD-Passwd", "admin")
	res, err = client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode == 200 {
		return fmt.Errorf("in order to use cloud, it's required to set a new strong password (follow the change passwords instructions: %s)", checkPasswordsWikiUrl)
	} else if res.StatusCode != 403 {
		return fmt.Errorf("weird status code %d for %s with default password", res.StatusCode, authCheckUrl)
	}
	return nil
}

func restoreNginx(nginxAffected bool) {
	if !nginxAffected {
		return
	}
	logrus.Info("Reverting nginx http configuration for letsencrypt...")
	if err := os.WriteFile(nginxFilepath, nginxHttpContent, 0644); err != nil {
		logrus.WithError(err).Error("unable to write nginx configuration")
		return
	}
	if err := launchCmd([]string{"systemctl", "restart", "kvmd-nginx"}); err != nil {
		logrus.WithError(err).Error("unable to restart nginx")
		return
	}
}
