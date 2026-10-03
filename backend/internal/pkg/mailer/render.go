package mailer

import (
	"bytes"
	"embed"
	"fmt"
	htmltemplate "html/template"
	"io/fs"
	"strings"
	"sync"
	texttemplate "text/template"
)

// templatesFS holds the email templates. Convention: every email <name> has
// a <name>.txt.tmpl defining "subject" and "text" blocks, and optionally a
// <name>.html.tmpl defining a "content" block that is wrapped by the
// shared layout.html.tmpl ("layout" block).
//
//go:embed templates/*.tmpl
var templatesFS embed.FS

const (
	layoutTemplateFile = "templates/layout.html.tmpl"
	textTemplateSuffix = ".txt.tmpl"
	htmlTemplateSuffix = ".html.tmpl"
)

// VerifyEmailData is the template data for the "verify_email" email.
type VerifyEmailData struct {
	Name      string
	VerifyURL string
	TTLHours  int
}

type emailTemplate struct {
	text *texttemplate.Template
	html *htmltemplate.Template // nil when the email has no HTML part
}

var (
	templatesOnce sync.Once
	templates     map[string]emailTemplate
	templatesErr  error
)

// loadTemplates parses all embedded templates once. A parse error is kept
// and returned by every Render call (and surfaces in tests) instead of
// panicking at package init.
func loadTemplates() (map[string]emailTemplate, error) {
	templatesOnce.Do(func() {
		templates, templatesErr = parseTemplates()
	})
	return templates, templatesErr
}

func parseTemplates() (map[string]emailTemplate, error) {
	textFiles, err := fs.Glob(templatesFS, "templates/*"+textTemplateSuffix)
	if err != nil {
		return nil, fmt.Errorf("mailer: list templates: %w", err)
	}
	result := make(map[string]emailTemplate, len(textFiles))
	for _, textFile := range textFiles {
		name := strings.TrimSuffix(strings.TrimPrefix(textFile, "templates/"), textTemplateSuffix)

		textTmpl, err := texttemplate.ParseFS(templatesFS, textFile)
		if err != nil {
			return nil, fmt.Errorf("mailer: parse %s: %w", textFile, err)
		}
		for _, block := range []string{"subject", "text"} {
			if textTmpl.Lookup(block) == nil {
				return nil, fmt.Errorf("mailer: %s does not define %q", textFile, block)
			}
		}
		entry := emailTemplate{text: textTmpl}

		htmlFile := "templates/" + name + htmlTemplateSuffix
		if _, err := fs.Stat(templatesFS, htmlFile); err == nil {
			htmlTmpl, err := htmltemplate.ParseFS(templatesFS, layoutTemplateFile, htmlFile)
			if err != nil {
				return nil, fmt.Errorf("mailer: parse %s: %w", htmlFile, err)
			}
			if htmlTmpl.Lookup("layout") == nil || htmlTmpl.Lookup("content") == nil {
				return nil, fmt.Errorf("mailer: %s must define \"content\" (layout defines \"layout\")", htmlFile)
			}
			entry.html = htmlTmpl
		}
		result[name] = entry
	}
	return result, nil
}

// Render renders the named email template with data and returns a Message
// with Subject, TextBody and (when the email has an HTML template) HTMLBody
// set. The caller sets To. The HTML part uses html/template, so user
// provided values in data are escaped.
func Render(name string, data any) (Message, error) {
	set, err := loadTemplates()
	if err != nil {
		return Message{}, err
	}
	tmpl, ok := set[name]
	if !ok {
		return Message{}, fmt.Errorf("mailer: unknown template %q", name)
	}

	var subject, text bytes.Buffer
	if err := tmpl.text.ExecuteTemplate(&subject, "subject", data); err != nil {
		return Message{}, fmt.Errorf("mailer: render %s subject: %w", name, err)
	}
	if err := tmpl.text.ExecuteTemplate(&text, "text", data); err != nil {
		return Message{}, fmt.Errorf("mailer: render %s text: %w", name, err)
	}
	msg := Message{
		Subject:  strings.TrimSpace(subject.String()),
		TextBody: strings.TrimSpace(text.String()) + "\n",
	}

	if tmpl.html != nil {
		var html bytes.Buffer
		if err := tmpl.html.ExecuteTemplate(&html, "layout", data); err != nil {
			return Message{}, fmt.Errorf("mailer: render %s html: %w", name, err)
		}
		msg.HTMLBody = html.String()
	}
	return msg, nil
}
