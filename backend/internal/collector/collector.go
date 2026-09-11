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

// ParseEtcHosts convertit la sortie de `cat /etc/hosts` en entrées
// exploitables pour la reconstruction des groupes. Lignes vides,
// commentaires, localhost et loopback ignorés ; seul le premier hostname
// de chaque ligne est retenu.
func ParseEtcHosts(catOutput string) []model.EtcHostsEntry {
	var out []model.EtcHostsEntry
	for _, line := range strings.Split(catOutput, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		ip, hostname := fields[0], fields[1]
		if hostname == "" || strings.EqualFold(hostname, "localhost") {
			continue
		}
		if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "127.") {
			continue
		}
		out = append(out, model.EtcHostsEntry{IP: ip, Hostname: hostname})
	}
	return out
}

// HostData regroupe tout ce que la collecte SSH rapporte pour une VM.
type HostData struct {
	Apps     []model.AppVersion
	EtcHosts []model.EtcHostsEntry
}

// Dial établit une connexion SSH (clé privée) vers l'hôte donné.
// Exporté pour les exécuteurs distants (ex: virsh via SSH vers un hôte KVM).
func Dial(ctx context.Context, ip, user string, port int, keyPath string, timeout time.Duration) (*ssh.Client, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("clé ssh: %w", err)
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("clé ssh invalide: %w", err)
	}
	if port <= 0 {
		port = 22
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := (&net.Dialer{Timeout: timeout}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("connexion ssh %s: %w", addr, err)
	}
	c, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         timeout,
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("auth ssh %s: %w", addr, err)
	}
	return ssh.NewClient(c, chans, reqs), nil
}

// runCmd exécute une commande distante et retourne sa sortie combinée.
func runCmd(ctx context.Context, client *ssh.Client, cmd string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	s, err := client.NewSession()
	if err != nil {
		return "", err
	}
	defer s.Close()
	out, err := s.CombinedOutput(cmd)
	if err != nil {
		return "", fmt.Errorf("%s: %w", cmd, err)
	}
	return string(out), nil
}

// CollectFull se connecte en SSH à la VM et rapporte versions
// d'applications + entrées /etc/hosts. Un dossier absent ou un /etc/hosts
// illisible n'échoue pas toute la collecte (données partielles).
func CollectFull(ctx context.Context, ip string, sshCfg config.SSHConfig, dirs []string) (HostData, error) {
	var data HostData
	if len(dirs) == 0 {
		dirs = DefaultAppDirs()
	}
	timeout := time.Duration(sshCfg.TimeoutSeconds) * time.Second
	client, err := Dial(ctx, ip, sshCfg.User, sshCfg.Port, sshCfg.PrivateKeyPath, timeout)
	if err != nil {
		return data, err
	}
	defer client.Close()

	for _, dir := range dirs {
		out, err := runCmd(ctx, client, "ls -1 -- "+shellQuote(dir))
		if err != nil {
			continue
		}
		data.Apps = append(data.Apps, ParseAppVersions(out)...)
	}
	if out, err := runCmd(ctx, client, "cat /etc/hosts"); err == nil {
		data.EtcHosts = ParseEtcHosts(out)
	}
	return data, nil
}

// Collect se connecte en SSH à la VM et liste les dossiers donnés.
// Un dossier absent est ignoré (pas d'erreur globale).
func Collect(ctx context.Context, ip string, sshCfg config.SSHConfig, dirs []string) ([]model.AppVersion, error) {
	data, err := CollectFull(ctx, ip, sshCfg, dirs)
	if err != nil {
		return nil, err
	}
	return data.Apps, nil
}
