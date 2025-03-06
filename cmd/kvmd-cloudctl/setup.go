package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tmaxmax/go-sse"
	"golang.org/x/term"
	"google.golang.org/protobuf/types/known/emptypb"
	"gopkg.in/yaml.v3"

	"github.com/pikvm/cloud-api/api_models"
	hiveagent_pb "github.com/pikvm/cloud-api/proto/hiveagent"
	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/hive"
)

const (
	authFilepath          = "/etc/kvmd/cloud/auth.yaml"
	nginxFilepath         = "/etc/kvmd/cloud/nginx.ctx-http.conf"
	baseDomain            = "pikvm.cloud"
	authCheckUrl          = "https://localhost/api/auth/check"
	checkPasswordsWikiUrl = "https://docs.pikvm.org/first_steps/#getting-access-to-pikvm"
)

//go:embed configs/nginx.http.conf
var nginxHttpContent []byte

//go:embed configs/nginx.http-and-https.conf
var nginxHttpsContent []byte

func Setup(cmd *cobra.Command, args []string) error {
	if err := checkLocalAuth(); err != nil {
		return fmt.Errorf("local kvmd auth check failed: %w", err)
	}

	token, err := obtainCreds(cmd.Context())
	if err != nil {
		return err
	}
	config.Cfg.AuthToken = token

	logrus.Info("Performing a cloud connection attempt...")
	info, err := tryAuthorize(cmd.Context())
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}
	logrus.Info("Authorization successful")

	if err := saveAuthData(); err != nil {
		return fmt.Errorf("unable to save authorization data: %w", err)
	}
	logrus.Info("Authorization information saved")

	httpRouters := info.GetRouters().GetHttpRouters()
	var nginxAffected bool = false
	if len(httpRouters) == 0 {
		logrus.Warnf("No http routers found. Skipping letsencrypt setup")
	} else {
		fqdn := httpRouters[0].GetFqdn()
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
			"--email", info.GetUser().GetEmail(),
			"-d", fqdn,
		}); err != nil {
			return fmt.Errorf("unable to get certificate: %w", err)
		}
		nginxAffected = true
		defer func() { restoreNginx(nginxAffected) }()
		if err := os.WriteFile(nginxFilepath, nginxHttpsContent, 0664); err != nil {
			return fmt.Errorf("unable to write nginx configuration: %w", err)
		}
		if err := launchCmd([]string{"kvmd-certbot", "install_cloud", fqdn}); err != nil {
			return fmt.Errorf("unable to install certificate: %w", err)
		}
		if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-certbot.timer"}); err != nil {
			return fmt.Errorf("unable to install certificate: %w", err)
		}

		logrus.Info("Your system is accessible externally via https://" + fqdn)
	}

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
	url, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/agent/bootstrap")
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var token string
	var done bool
	var eventError error

	r, err := http.NewRequestWithContext(ctx, "GET", url, http.NoBody)
	if err != nil {
		return "", err
	}
	conn := sse.NewConnection(r)

	conn.SubscribeEvent("redirect", func(e sse.Event) {
		redirect := api_models.BootstrapRedirect{}
		err := json.Unmarshal([]byte(e.Data), &redirect)
		if err != nil {
			eventError = err
			cancel()
			return
		}
		fmt.Printf("Please, open the following URL in your browser and follow instructions: %s\n", redirect.RedirectURL)
	})

	conn.SubscribeEvent("result", func(e sse.Event) {
		result := api_models.BootstrapResult{}
		err := json.Unmarshal([]byte(e.Data), &result)
		if err != nil {
			eventError = err
			cancel()
			return
		}
		token = result.AuthToken
		done = true
		cancel()
	})

	err = conn.Connect()
	if eventError != nil {
		return "", eventError
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

func tryAuthorize(ctx context.Context) (*hiveagent_pb.AgentInfo, error) {
	authCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	conn, err := hive.Dial(authCtx)
	if err != nil {
		return nil, err
	}
	info, err := conn.Client.WhoAmI(authCtx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return info, nil
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
