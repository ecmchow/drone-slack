package main

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/bluele/slack"
	"github.com/drone/drone-template-lib/template"
)

type (
	Repo struct {
		Owner        string
		Name         string
		Link         string
		HostInternal string
		HostExternal string
	}

	Build struct {
		Tag      string
		Event    string
		Number   int
		Parent   int
		Commit   string
		Ref      string
		Branch   string
		Author   Author
		Pull     string
		Message  Message
		DeployTo string
		Status   string
		Link     string
		Started  int64
		Created  int64
	}

	Author struct {
		Username string
		Name     string
		Email    string
		Avatar   string
	}

	Message struct {
		msg   string
		Title string
		Body  string
	}

	Config struct {
		Webhook      string
		Channel      string
		Recipient    string
		Username     string
		Template     string
		Fallback     string
		ImageURL     string
		IconURL      string
		IconEmoji    string
		HostInternal string
		HostExternal string
		Color        string
		LinkNames    bool
	}

	Job struct {
		Started int64
	}

	Plugin struct {
		Repo   Repo
		Build  Build
		Config Config
		Job    Job
	}
)

func (a Author) String() string {
	return a.Username
}

func newCommitMessage(m string) Message {
	// not checking the length here
	// as split will always return at least one element
	// check it if using more than the first element
	splitMsg := strings.Split(m, "\n")

	return Message{
		msg:   m,
		Title: strings.TrimSpace(splitMsg[0]),
		Body:  strings.TrimSpace(strings.Join(splitMsg[1:], "\n")),
	}
}
func (m Message) String() string {
	return m.msg
}

func (p Plugin) Exec() error {
	attachment := slack.Attachment{
		Color:      p.Config.Color,
		ImageURL:   p.Config.ImageURL,
		MarkdownIn: []string{"text", "fallback"},
	}
	if p.Config.Color == "" {
		attachment.Color = color(p.Build)
	}
	if p.Config.Fallback != "" {
		f, err := templateMessage(p.Config.Fallback, p)
		if err != nil {
			return err
		}
		attachment.Fallback = f
	} else {
		attachment.Fallback = fallback(p.Repo, p.Build)
	}

	payload := slack.WebHookPostPayload{}
	payload.Username = p.Config.Username
	payload.Attachments = []*slack.Attachment{&attachment}
	payload.IconUrl = p.Config.IconURL
	payload.IconEmoji = p.Config.IconEmoji

	if p.Config.Recipient != "" {
		payload.Channel = prepend("@", p.Config.Recipient)
	} else if p.Config.Channel != "" {
		payload.Channel = prepend("#", p.Config.Channel)
	}
	if p.Config.LinkNames {
		payload.LinkNames = "1"
	}
	if p.Config.Template != "" {
		var err error
		attachment.Text, err = templateMessage(p.Config.Template, p)
		if err != nil {
			return err
		}
	} else {
		attachment.Text = message(p.Repo, p.Build, p.Config)
	}

	client := slack.NewWebHook(p.Config.Webhook)
	return client.PostMessage(&payload)
}

func templateMessage(t string, plugin Plugin) (string, error) {
	return template.RenderTrim(t, plugin)
}

func message(repo Repo, build Build, config Config) string {
	statusIcon := icon(build)
	msgTitle := fmt.Sprintf("*%s %s %s*", statusIcon, strings.Title(build.Event), strings.ToUpper(build.Status))
	msgRepo := fmt.Sprintf("Repo: `%s/%s` (%s)", repo.Owner, repo.Name, build.Branch)
	msgBuild := fmt.Sprintf("Build #%d (%s) by %s", build.Number, build.Commit[:8], build.Author)
	msgFooter := fmt.Sprintf("<%s|Drone CI>", build.Link)

	if build.Event == "promote" && build.DeployTo != "" {
		msgTitle = fmt.Sprintf("*%s %s Deploy %s*", statusIcon, strings.Title(build.DeployTo), strings.ToUpper(build.Status))
	} else if build.DeployTo != "" {
		msgTitle = fmt.Sprintf("*%s %s %s %s*", statusIcon, strings.Title(build.DeployTo), strings.Title(build.Event), strings.ToUpper(build.Status))
	}

	if config.HostInternal != "" && config.HostExternal != "" {
		urlInternal := ""
		urlExternal := ""

		url, err := url.Parse(build.Link)
		if err != nil {
			urlInternal = build.Link
			urlExternal = build.Link
		} else {
			urlInternal = strings.Replace(build.Link, url.Hostname(), config.HostInternal, 1)
			urlExternal = strings.Replace(build.Link, url.Hostname(), config.HostExternal, 1)
		}

		msgFooter = fmt.Sprintf("<%s|Drone CI> (<%s|External>)", urlInternal, urlExternal)
	}

	return fmt.Sprintf(
		"%s\n%s\n%s\n%s",
		msgTitle,
		msgRepo,
		msgBuild,
		msgFooter,
	)
}

func fallback(repo Repo, build Build) string {
	return fmt.Sprintf("%s %s/%s#%s (%s) by %s",
		build.Status,
		repo.Owner,
		repo.Name,
		build.Commit[:8],
		build.Branch,
		build.Author,
	)
}

func color(build Build) string {
	switch build.Status {
	case "success":
		return "good"
	case "failure", "error", "killed":
		return "danger"
	default:
		return "warning"
	}
}

func icon(build Build) string {
	switch build.Status {
	case "success":
		return ":white_check_mark:"
	case "failure", "error", "killed":
		return ":x:"
	default:
		return ":warning:"
	}
}

func prepend(prefix, s string) string {
	if !strings.HasPrefix(s, prefix) {
		return prefix + s
	}

	return s
}
