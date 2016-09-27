package main

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/BurntSushi/toml"
	"github.com/dirker/gitlab-mirror/git"
	"github.com/mattn/go-shellwords"
	"github.com/xanzy/go-gitlab"
)

const (
	configFile = "gitlab-mirror.conf"
)

type Config struct {
	Gitlab         string   `toml:"gitlab"`
	GitlabAPI      string   `toml:"gitlab_api"`
	GitlabToken    string   `toml:"gitlab_token"`
	RepositoryPath string   `toml:"repository_path"`
	Repos          []string `toml:"repos"`
}

type Context struct {
	config *Config
	gitlab *gitlab.Client
}

func LoadConfig() (c *Config, err error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	c = new(Config)
	_, err = toml.Decode(string(data), c)
	if err != nil {
		return nil, err
	}

	if c.GitlabAPI == "" {
		c.GitlabAPI = c.Gitlab + "/api/v3"
	}
	c.RepositoryPath = os.ExpandEnv(c.RepositoryPath)

	return
}

func getKeys(client *gitlab.Client) (userKeysMap map[int][]string, err error) {
	users, response, err := client.Users.ListUsers(nil)
	if err != nil {
		panic(err)
	}

	for response.NextPage != 0 {
		var u []*gitlab.User
		u, response, err = client.Users.ListUsers(
			&gitlab.ListUsersOptions{
				ListOptions: gitlab.ListOptions{
					Page: response.NextPage,
				},
			},
		)
		users = append(users, u...)
	}

	userKeysMap = make(map[int][]string)

	for _, u := range users {
		keys, _, err := client.Users.ListSSHKeysForUser(u.ID)
		if err != nil {
			panic(err)
		}

		pubkeys := make([]string, len(keys))
		for i, k := range keys {
			pubkeys[i] = k.Key
		}

		userKeysMap[u.ID] = pubkeys
	}

	return
}

func syncKeys(ctx *Context, args []string) {
	userKeysMap, _ := getKeys(ctx.gitlab)

	f, err := ioutil.TempFile("", "authorized_keys")
	if err != nil {
		panic(err)
	}

	options := []string{
		"no-port-forwarding",
		"no-X11-forwarding",
		"no-agent-forwarding",
		"no-pty",
	}

	for userId, pubkeys := range userKeysMap {
		command := fmt.Sprintf("command=\"gitlab-mirror serve %d\"", userId)
		o := command + "," + strings.Join(options, ",")

		for _, pubkey := range pubkeys {
			fmt.Fprintf(f, "%s %s\n", o, pubkey)
		}
	}

	f.Close()
	os.Rename(f.Name(), ".ssh/authorized_keys")
}

func getProjects(client *gitlab.Client) (projects []*gitlab.Project, err error) {
	var response *gitlab.Response

	lo := gitlab.ListOptions{
		Page:    1,
		PerPage: 100,
	}

	projects, response, err = client.Projects.ListProjects(
		&gitlab.ListProjectsOptions{
			ListOptions: lo,
		},
	)

	if err != nil {
		panic(err)
	}

	for response.NextPage != 0 {
		var p []*gitlab.Project
		lo.Page = response.NextPage
		p, response, err = client.Projects.ListProjects(
			&gitlab.ListProjectsOptions{
				ListOptions: lo,
			},
		)
		projects = append(projects, p...)
	}

	return
}

func isProjectMirrored(p *gitlab.Project, patterns []string) bool {
	for _, pattern := range patterns {
		path := p.PathWithNamespace
		if strings.HasPrefix(path, pattern) {
			return true
		}
	}

	return false
}

func isBareRepo(repoPath string) bool {
	fi, err := os.Stat(repoPath)
	if err != nil {
		return false
	}

	if !fi.IsDir() {
		return false
	}

	_, err = git.ExecIn(repoPath, "rev-parse", "--is-bare-repository")
	return err == nil
}

func repoUpdate(ctx *Context, repoPath string) {
	origin, _ := url.Parse(ctx.config.Gitlab)

	if !strings.HasSuffix(repoPath, ".git") {
		repoPath += ".git"
	}

	origin.Scheme = "ssh"
	origin.User = url.User("git")
	origin.Path = repoPath

	repo := filepath.Join(ctx.config.RepositoryPath, repoPath)

	if isBareRepo(repo) {
		_, err := git.ExecIn(repo, "remote", "set-url", "origin", origin.String())
		if err != nil {
			panic(err)
		}
		_, err = git.ExecIn(repo, "fetch")
		if err != nil {
			panic(err)
		}
	} else {
		os.RemoveAll(repo)
		_, err := git.Exec("clone", "--mirror", origin.String(), repo)
		if err != nil {
			panic(err)
		}
	}
}

func syncRepos(ctx *Context, args []string) {
	projects, _ := getProjects(ctx.gitlab)
	patterns := ctx.config.Repos

	filtered := make([]*gitlab.Project, 0)
	for _, p := range projects {
		if isProjectMirrored(p, patterns) {
			filtered = append(filtered, p)
		}
	}

	for _, p := range filtered {
		fmt.Printf("%04d: %s\n", p.ID, p.PathWithNamespace)
		repoUpdate(ctx, p.PathWithNamespace)
	}
}

func sync(ctx *Context, args []string) {
	syncKeys(ctx, args)
	syncRepos(ctx, args)
}

func serve(ctx *Context, args []string) {
	if len(args) != 1 {
		panic("need user id")
	}

	client := ctx.gitlab

	userId, err := strconv.Atoi(args[0])
	if err != nil {
		panic(err)
	}

	user, _, err := client.Users.GetUser(userId)
	if err != nil {
		/* FIXME: gracefully handle user deletion */
		panic(err)
	}

	origCmd := os.Getenv("SSH_ORIGINAL_COMMAND")

	//fmt.Printf("Hello %s, you are trying to execute:\n", user.Username)
	//fmt.Printf("  '%s'\n", origCmd)

	if user.State != "active" {
		os.Stderr.WriteString("user not active\n")
		os.Exit(1)
	}

	args, err = shellwords.Parse(origCmd)
	if err != nil {
		panic(err)
	}
	if len(args) == 0 {
		os.Stderr.WriteString("interactive mode not supported\n")
		os.Exit(1)
	}

	cmd := args[0]

	/* FIXME: allow other commands, too */
	if cmd != "git-upload-pack" {
		os.Stderr.WriteString("command not supported\n")
		os.Exit(1)
	}

	if len(args) != 2 {
		os.Stderr.WriteString("no repo specified\n")
		os.Exit(1)
	}

	projectPath := args[1]
	projectPath = strings.Replace(projectPath, "'", "", -1)
	projectPath = strings.TrimPrefix(projectPath, "/")
	projectPath = strings.TrimSuffix(projectPath, ".git")

	repoPath := filepath.Join(ctx.config.RepositoryPath, projectPath)
	repoPath += ".git"

	if !isBareRepo(repoPath) {
		os.Stderr.WriteString("Repository does not exist, please check the path.\n")
		os.Exit(1)
	}

	project, _, err := client.Projects.GetProject(projectPath)
	if err != nil {
		panic(err)
	}

	if project.VisibilityLevel == gitlab.PrivateVisibility {
		if !user.IsAdmin {
			// Validate access rights on member level
			var member *gitlab.ProjectMember
			member, _, err = client.Projects.GetProjectMember(project.ID, userId)
			if err != nil {
				os.Stderr.WriteString("not member of project\n")
				os.Exit(1)
			}

			if member.AccessLevel < gitlab.ReporterPermissions {
				os.Stderr.WriteString("not authorized\n")
				os.Exit(1)
			}
		}
	}

	// permissions ok, execute cmd with args

	binary, err := exec.LookPath("git-upload-pack")
	if err != nil {
		panic(err)
	}
	args = []string{
		cmd,
		repoPath,
	}

	// FIXME: strip down environment
	env := os.Environ()
	err = syscall.Exec(binary, args, env)
	if err != nil {
		panic(err)
	}
}

type CommandFunc func(ctx *Context, args []string)

var (
	commands = map[string]CommandFunc{
		"serve":      serve,
		"sync":       sync,
		"sync-keys":  syncKeys,
		"sync-repos": syncRepos,
	}
)

func usage() {
	os.Stderr.WriteString("usage: gitlab-mirror <cmd> [args...]\n")
}

func main() {
	//fmt.Println(strings.Join(os.Args, " "))

	config, err := LoadConfig()
	if err != nil {
		panic("error loading configuration file")
	}

	gl := gitlab.NewClient(nil, config.GitlabToken)
	gl.SetBaseURL(config.GitlabAPI)

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	cmd, args := os.Args[1], os.Args[2:]

	ctx := &Context{
		config: config,
		gitlab: gl,
	}

	handler, ok := commands[cmd]
	if !ok {
		usage()
		os.Exit(1)
	}

	handler(ctx, args)
}
