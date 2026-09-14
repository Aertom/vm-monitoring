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
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/Aertom/vm-monitoring/backend/internal/config"
	"github.com/Aertom/vm-monitoring/backend/internal/model"
)

// DirsForFamily retourne les dossiers à scruter pour une famille donnée
// (clé insensible à la casse), ou rien si non configurée (pas de collecte).
func DirsForFamily(cfg *config.Config, family string) []string {
	if cfg != nil {
		key := strings.ToLower(strings.TrimSpace(family))
		if dirs := cfg.AppDirs[key]; len(dirs) > 0 {
			return dirs
		}
	}
	return nil
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
		name, version := splitVersion(line)
		apps = append(apps, model.AppVersion{Name: name, Version: version})
	}
	return apps
}

// splitVersion découpe "nom_version" (ou "nom-version") sur le dernier
// séparateur suivi d'un chiffre. Sans correspondance, version vide.
func splitVersion(entry string) (name, version string) {
	name, version = entry, ""
	if m := versionSplit.FindStringSubmatch(entry); m != nil {
		name, version = m[1], m[2]
	}
	return name, version
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

// expandPath résout le préfixe ~ vers le home directory (os.ReadFile ne
// le fait pas). Utile pour privateKeyPath: ~/.ssh/id_rsa en local.
func expandPath(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
	}
	return p
}

// Dial établit une connexion SSH vers l'hôte donné (clé privée et/ou mot
// de passe ; au moins une méthode requise).
// Exporté pour les exécuteurs distants (ex: virsh via SSH vers un hôte KVM).
func Dial(ctx context.Context, ip, user string, port int, keyPath, password string, timeout time.Duration) (*ssh.Client, error) {
	var auths []ssh.AuthMethod
	if keyPath = expandPath(keyPath); keyPath != "" {
		key, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("clé ssh: %w", err)
		}
		signer, err := ssh.ParsePrivateKey(key)
		if err != nil {
			return nil, fmt.Errorf("clé ssh invalide: %w", err)
		}
		auths = append(auths, ssh.PublicKeys(signer))
	}
	if password != "" {
		auths = append(auths, ssh.Password(password))
		// Certains serveurs (ESXi, équipements) n'annoncent que
		// keyboard-interactive : on y répond par le mot de passe.
		auths = append(auths, ssh.KeyboardInteractive(
			func(user, instruction string, questions []string, echos []bool) ([]string, error) {
				answers := make([]string, len(questions))
				for i := range answers {
					answers[i] = password
				}
				return answers, nil
			}))
	}
	if len(auths) == 0 {
		return nil, fmt.Errorf("ssh: aucune méthode d'authentification (clé ou mot de passe)")
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
		Auth:            auths,
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

// ResolveAuth détermine user/mot de passe effectifs : valeurs par VM
// prioritaires, sinon globales.
func ResolveAuth(staticUser, staticPass string, global config.SSHConfig) (string, string) {
	user := global.User
	if staticUser != "" {
		user = staticUser
	}
	pass := global.Password
	if staticPass != "" {
		pass = staticPass
	}
	return user, pass
}

// SSHConfigured indique si une collecte SSH est possible : clé configurée
// ou mot de passe (global ou par VM).
func SSHConfigured(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	if cfg.SSH.PrivateKeyPath != "" || cfg.SSH.Password != "" {
		return true
	}
	for _, sv := range cfg.StaticVMs {
		if sv.SSHPassword != "" {
			return true
		}
	}
	return false
}

// SSHExecutor exécute des commandes sur un hôte distant via SSH.
// Clé privée et/ou mot de passe. Signature compatible avec
// kvm.CommandExecutor (sans en dépendre).
type SSHExecutor struct {
	Host     string
	User     string
	Port     int
	KeyPath  string
	Password string
	Timeout  time.Duration
}

// Run exécute name + args sur l'hôte distant (une connexion par appel).
func (e *SSHExecutor) Run(ctx context.Context, name string, args ...string) (string, error) {
	client, err := Dial(ctx, e.Host, e.User, e.Port, e.KeyPath, e.Password, e.Timeout)
	if err != nil {
		return "", err
	}
	defer client.Close()
	parts := append([]string{name}, args...)
	quoted := make([]string, 0, len(parts))
	for _, p := range parts {
		quoted = append(quoted, shellQuote(p))
	}
	out, err := runCmd(ctx, client, strings.Join(quoted, " "))
	if err != nil {
		return "", fmt.Errorf("%s: %w", e.Host, err)
	}
	return out, nil
}

// CollectFull se connecte en SSH à la VM et rapporte versions
// d'applications + entrées /etc/hosts. Sans dossiers configurés, seules
// les entrées /etc/hosts sont collectées (pas de versions). Un dossier
// absent ou un /etc/hosts illisible n'échoue pas toute la collecte.
func CollectFull(ctx context.Context, ip string, sshCfg config.SSHConfig, dirs []string) (HostData, error) {
	var data HostData
	timeout := time.Duration(sshCfg.TimeoutSeconds) * time.Second
	client, err := Dial(ctx, ip, sshCfg.User, sshCfg.Port, sshCfg.PrivateKeyPath, sshCfg.Password, timeout)
	if err != nil {
		return data, err
	}
	defer client.Close()

	for _, dir := range dirs {
		out, err := runCmd(ctx, client, "ls -1 -- "+shellQuote(dir))
		if err != nil {
			continue
		}
		for _, entry := range splitLines(out) {
			display := entry
			if target, ok := tryReadlink(ctx, client, dir, entry); ok {
				display = target
			}
			data.Apps = append(data.Apps, appForEntry(entry, display))
		}
	}
	if out, err := runCmd(ctx, client, "cat /etc/hosts"); err == nil {
		data.EtcHosts = ParseEtcHosts(out)
	}
	return data, nil
}

// splitLines découpe une sortie multi-lignes en entrées non vides.
func splitLines(out string) []string {
	var entries []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			entries = append(entries, line)
		}
	}
	return entries
}

// tryReadlink résout un lien symbolique : retourne la cible brute si
// dir/entry est un lien, false sinon (ou en cas d'erreur).
func tryReadlink(ctx context.Context, client *ssh.Client, dir, entry string) (string, bool) {
	if strings.Contains(entry, "/") {
		return "", false
	}
	out, err := runCmd(ctx, client, "readlink -- "+shellQuote(dir+"/"+entry))
	if err != nil {
		return "", false
	}
	if target := strings.TrimSpace(out); target != "" {
		return target, true
	}
	return "", false
}

// appForEntry construit l'application affichée : si l'entrée est un lien
// symbolique, on affiche le nom du dossier/fichier cible (ex: lien
// /opt/appli1 -> /appli/appli_1.2.3 affiché "appli_1.2.3"), sinon on
// décompose "nom_version" classiquement.
func appForEntry(entry, display string) model.AppVersion {
	if display != entry {
		return model.AppVersion{Name: path.Base(display)}
	}
	name, version := splitVersion(entry)
	return model.AppVersion{Name: name, Version: version}
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
