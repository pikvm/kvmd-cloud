package main

import (
	"context"
	"crypto/tls"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/pikvm/kvmd-cloud/internal/config"
	"github.com/pikvm/kvmd-cloud/internal/hive"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
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
	log.Info("Checking local authorization status")
	if err := checkLocalAuth(); err != nil {
		return fmt.Errorf("local auth check failed: %w", err)
	}

	agentName, token, domainName, email, err := askCreds()
	if err != nil {
		return err
	}
	config.Cfg.AgentName = agentName
	config.Cfg.AuthToken = token

	log.Info("Performing authorization attempt...")
	if err := tryAuthorize(cmd.Context()); err != nil {
		return fmt.Errorf("authorization failed: %w", err)
	}
	log.Info("Authorization successful")

	if err := saveAuthData(); err != nil {
		return fmt.Errorf("unable to save authorization data: %w", err)
	}
	log.Info("Authorization information saved")

	log.Info("Preparing http configuration for letsencrypt...")
	if err := os.WriteFile(nginxFilepath, nginxHttpContent, 0644); err != nil {
		return fmt.Errorf("unable to write nginx configuration: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "restart", "kvmd-nginx"}); err != nil {
		return fmt.Errorf("unable to restart nginx: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-cloud"}); err != nil {
		return fmt.Errorf("unable to start kvmd-cloud agent: %w", err)
	}
	log.Info("Requesting letsencrypt SSL certificate...")
	if err := launchCmd([]string{
		"kvmd-certbot", "certonly_webroot", "--agree-tos", "-n",
		"--email", email,
		"-d", domainName,
	}); err != nil {
		return fmt.Errorf("unable to get certificate: %w", err)
	}
	nginxAffected := true
	defer restoreNginx(nginxAffected)
	if err := os.WriteFile(nginxFilepath, nginxHttpsContent, 0664); err != nil {
		return fmt.Errorf("unable to write nginx configuration: %w", err)
	}
	if err := launchCmd([]string{"kvmd-certbot", "install_cloud", domainName}); err != nil {
		return fmt.Errorf("unable to install certificate: %w", err)
	}
	if err := launchCmd([]string{"systemctl", "enable", "--now", "kvmd-certbot.timer"}); err != nil {
		return fmt.Errorf("unable to install certificate: %w", err)
	}

	log.Info("Done. Please, ensure that you password is strong enough")
	log.Info("Your system is accessible externally via https://" + domainName)

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

func askCreds() (agentName string, token string, domainName string, email string, err error) {
	fmt.Println("Input authorization data:")

	fmt.Print("Agent name: ")
	if _, err = fmt.Scanln(&agentName); err != nil {
		return
	}

	fmt.Print("Authorization token: ")
	var b []byte
	b, err = term.ReadPassword(int(syscall.Stdin))
	fmt.Println()
	if err != nil {
		return
	}
	token = string(b)

	domainName = agentName + ".test." + baseDomain
	// fmt.Print("Domain name: ")
	// if _, err = fmt.Scanln(&domainName); err != nil {
	// 	return
	// }

	fmt.Print("Email address: ")
	if _, err = fmt.Scanln(&email); err != nil {
		return
	}

	return
}

func tryAuthorize(ctx context.Context) error {
	authCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	_, err := hive.Dial(authCtx)
	if err != nil {
		return err
	}
	return nil
}

func saveAuthData() error {
	authFileContent := struct {
		AgentName string `yaml:"agent_name"`
		AuthToken string `yaml:"auth_token"`
	}{
		AgentName: config.Cfg.AgentName,
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
	// TODO:
}
