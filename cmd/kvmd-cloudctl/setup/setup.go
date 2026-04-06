package setup

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

	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"

	"github.com/pikvm/cloud-api/api_models"
	"github.com/pikvm/cloud-api/domain_errors"
	"github.com/pikvm/kvmd-cloud/internal/config"
)

const (
	authCheckUrl          = "https://localhost/api/auth/check"
	checkPasswordsWikiUrl = "https://docs.pikvm.org/first_steps/#getting-access-to-pikvm"
)

var (
	nginxFilepath = "/etc/kvmd/cloud/nginx.ctx-http.conf"
)

//go:embed configs/nginx.http.conf
var nginxHttpContent []byte

//go:embed configs/nginx.http-and-https.conf
var nginxHttpsContent []byte

func init() {
	if config.EnvIsHere {
		nginxFilepath = ".env/nginx.ctx-http.conf"
	}
}

func BuildCommand() *cli.Command {
	return &cli.Command{
		Name:  "setup",
		Usage: "Perform the initial setup and authorization of kvmd-cloud agent",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "ask-token",
				Usage: "Prompt for a token instead of using browser authentication",
			},
			&cli.BoolFlag{
				Name:   "skip-local-auth-check",
				Usage:  "Skip local authorization check. Use it only if you know what you are doing (e.g. for development or if you have a custom setup with disabled local auth)",
				Hidden: true,
			},
			&cli.BoolFlag{
				Name:   "skip-cert-setup",
				Usage:  "Skip SSL certificate setup. Use it only if you know what you are doing (e.g. for development or if you have a custom setup with disabled SSL)",
				Hidden: true,
			},
		},
		Action: Setup,
	}
}

func Setup(ctx context.Context, cmd *cli.Command) error {
	logger := log.Logger

	if !cmd.Bool("skip-local-auth-check") {
		if err := checkLocalAuth(); err != nil {
			return fmt.Errorf("local kvmd auth check failed: %w", err)
		}
	}

	askToken := cmd.Bool("ask-token")

	var token string = ""
	var err error = nil
	if !askToken {
		token, err = browserAuth(ctx)
		if err != nil {
			logger.Err(err).Msg("Browser authentication failed, falling back to token input")
		}
	}
	if err != nil || token == "" {
		token, err = askCreds(ctx)
		if ctx.Err() != nil {
			return nil
		}
	}
	if err != nil {
		return err
	}

	config.Cfg.AuthToken = token

	logger.Info().Msg("Performing a cloud connection attempt...")
	me, err := whoami(ctx, token)
	if err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}
	logger.Info().Msgf("Authorization successful. My name: %s/%s", me.User.Name, me.Name)

	if err := saveAuthData(); err != nil {
		return fmt.Errorf("unable to save authorization data: %w", err)
	}
	logger.Info().Msg("Authorization information saved")

	if cmd.Bool("skip-cert-setup") {
		logger.Info().Msg("Skipping certificate setup as requested. Make sure to set up SSL certificate for your system manually, otherwise your system won't be accessible externally and cloud features won't work")
		return nil
	}

	var nginxAffected bool = false
	logger.Info().Msg("Preparing http configuration for letsencrypt...")
	if err := os.WriteFile(nginxFilepath, nginxHttpContent, 0644); err != nil {
		return fmt.Errorf("unable to write nginx configuration: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "restart", "kvmd-nginx"}); err != nil {
		return fmt.Errorf("unable to restart nginx: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-cloud"}); err != nil {
		return fmt.Errorf("unable to start kvmd-cloud agent: %w", err)
	}
	logger.Info().Msg("Requesting letsencrypt SSL certificate...")
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

	logger.Info().Msg("Your system is accessible externally via https://" + me.DefaultFqdn)

	logger.Info().Msg("Done. Please, ensure that you password is strong enough")

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

func browserAuth(ctx context.Context) (string, error) {
	logger := log.Logger

	logger.Info().Msg("Obtaining bootstrap URL")
	logger.Debug().Msgf("Bootstrap request endpoint: %s", config.Cfg.Hive.Endpoint)
	reqUrl, err := url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agents/bootstrap")
	if err != nil {
		return "", err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, http.NoBody)
	if err != nil {
		return "", err
	}

	redirectResp, err := http.DefaultClient.Do(r)
	if err != nil {
		return "", err
	}
	defer redirectResp.Body.Close()
	respBytes, err := io.ReadAll(redirectResp.Body)
	if err != nil {
		return "", err
	}

	redirect := &api_models.BootstrapRedirect{}
	response := api_models.ResponseModel{Result: redirect}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		return "", err
	}
	if response.Error != nil {
		return "", errors.New(response.Error.Error())
	}

	logger.Debug().Interface("payload", redirect).Msg("received redirect event")
	fmt.Printf("Please, open the following URL in your browser and follow instructions: %s\n", redirect.RedirectURL)

	reqUrl, err = url.JoinPath(config.Cfg.Hive.Endpoint, "/api/agents/bootstrap/", redirect.BootstrapToken)
	if err != nil {
		return "", err
	}
	r, err = http.NewRequestWithContext(ctx, http.MethodGet, reqUrl, http.NoBody)
	if err != nil {
		return "", err
	}

	resultResp, err := http.DefaultClient.Do(r)
	if err != nil {
		logger.Err(err).Msg("failed to complete bootstrap request")
		return "", err
	}
	defer resultResp.Body.Close()
	respBytes, err = io.ReadAll(resultResp.Body)
	if err != nil {
		logger.Err(err).Msg("failed to read bootstrap completion response")
		return "", err
	}

	println(string(respBytes))

	result := &api_models.BootstrapResult{}
	response = api_models.ResponseModel{Result: result}
	if err := json.Unmarshal(respBytes, &response); err != nil {
		logger.Err(err).Msg("failed to parse bootstrap completion response")
		return "", err
	}
	if response.Error != nil {
		err = response.Error.ToDomainError()
		if errors.Is(err, domain_errors.ErrSessionExpired) {
			logger.Error().Msg("Authorization couldn't complete in time. Please, try again or input authorization token manually.")
		} else {
			logger.Error().Err(err).Msg("Failed to bootstrap agent")
		}
		return "", err
	}

	return result.AuthToken, nil
}

func askCreds(ctx context.Context) (token string, err error) {
	fmt.Print("Input authorization token: ")
	finishCh := make(chan struct{})
	defer close(finishCh)
	go func() {
		select {
		case <-ctx.Done():
			os.Exit(1)
		case <-finishCh:
		}
	}()
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
	if err = os.MkdirAll(filepath.Dir(config.AuthFilepath), 0755); err != nil {
		return err
	}
	if err = os.WriteFile(config.AuthFilepath, out, 0644); err != nil {
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
	logger := log.Logger

	if !nginxAffected {
		return
	}
	logger.Info().Msg("Reverting nginx http configuration for letsencrypt...")
	if err := os.WriteFile(nginxFilepath, nginxHttpContent, 0644); err != nil {
		logger.Err(err).Msg("unable to write nginx configuration")
		return
	}
	if err := launchCmd([]string{"systemctl", "restart", "kvmd-nginx"}); err != nil {
		logger.Err(err).Msg("unable to restart nginx")
		return
	}
}
