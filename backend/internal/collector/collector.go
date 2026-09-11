// Package collector récupère les versions des applications installées sur
// les VMs en listant via SSH les dossiers configurés par VM (appDirs).
// Convention : un dossier nommé "appli1_1.2.3" (ou "appli1-1.2.3") donne
// l'application "appli1" en version "1.2.3".
package collector

import (
	"context"
	"fmt"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// DefaultAppDirs est utilisé quand une famille ne configure aucun dossier.
func DefaultAppDirs() []string { return []string{"/opt"} }

// DirsForFamily retourne les dossiers à scruter pour une famille donnée
// (clé insensible à la casse), ou DefaultAppDirs si non configurée.
func DirsForFamily(cfg *config.Config, family string) []string {
	if cfg != nil {
		key := strings.ToLower(strings.TrimSpace(family))
		if dirs := cfg.AppDirs[key]; len(dirs) > 0 {
			return dirs
		}
	}
	return DefaultAppDirs()
}

// versionSplit découpe "nom_version" sur le dernier séparateur suivi d'un chiffre.
var versionSplit = regexp.MustCompile(`^(.+)[-_](\d.*)$`)

// ParseAppVersions convertit la sortie de `ls -1` en liste d'applications.
func ParseAppVersions(lsOutput string) []model.AppVersion {
	var apps []model.AppVersion
	for _, line := range strings.Split(lsOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		name, version := line, ""
		if m := versionSplit.FindStringSubmatch(line); m != nil {
			name, version = m[1], m[2]
		}
		apps = append(apps, model.AppVersion{Name: name, Version: version})
	}
	return apps
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Collect se connecte en SSH à la VM et liste les dossiers donnés.
// Un dossier absent est ignoré (pas d'erreur globale).
func Collect(ctx context.Context, ip string, sshCfg config.SSHConfig, dirs []string) ([]model.AppVersion, error) {
	if len(dirs) == 0 {
		dirs = DefaultAppDirs()
	}
	key, err := os.ReadFile(sshCfg.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("clé ssh: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("clé ssh invalide: %w", err)
	}
	port := sshCfg.Port
	if port <= 0 {
		port = 22
	}
	timeout := time.Duration(sshCfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connexion ssh %s: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            sshCfg.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth ssh %s: %w", addr, err)
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	var apps []model.AppVersion
	for _, dir := range dirs {
		out, err := runLs(ctx, client, dir)
		if err != nil {
			continue
		}
		apps = append(apps, ParseAppVersions(out)...)
	}
	return apps, nil
}

func runLs(ctx context.Context, client *ssh.Client, dir string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()
	out, err := s.CombinedOutput("ls -1 -- " + shellQuote(dir))
	if err != nil {
		return "", fmt.Errorf("ls %s: %w", dir, err)
	}
	return string(out), nil
}
