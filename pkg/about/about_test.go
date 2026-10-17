package about

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestFactsAgreeWithRepo(t *testing.T) {
	data, err := os.ReadFile("../../LICENSE")
	if err != nil {
		t.Fatal(err)
	}
	license := string(data)
	if !strings.HasPrefix(license, LicenseName) {
		t.Errorf("LICENSE doesn't start with %q", LicenseName)
	}
	if c := strings.Replace(Copyright, "©", "Copyright (c)", 1); !strings.Contains(license, c) {
		t.Errorf("LICENSE doesn't contain %q", c)
	}
	if Organization != "Nine Point Labs" || SupportEmail != "help@ninepointlabs.com" {
		t.Errorf("Organization = %q, SupportEmail = %q", Organization, SupportEmail)
	}
}

func TestMailtoURL(t *testing.T) {
	u, err := url.Parse(Info{Version: "1.2.3"}.MailtoURL())
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "mailto" || u.Opaque != SupportEmail {
		t.Errorf("got %s:%s, want mailto:%s", u.Scheme, u.Opaque, SupportEmail)
	}
	if strings.Contains(u.RawQuery, "+") {
		t.Errorf("spaces should be %%20, not +: %s", u.RawQuery)
	}
	q := u.Query()
	if got := q.Get("subject"); got != "Autonomix CLI 1.2.3" {
		t.Errorf("subject = %q", got)
	}
	if body := q.Get("body"); !strings.Contains(body, "Autonomix CLI 1.2.3\nGo ") {
		t.Errorf("body is missing version details: %q", body)
	}
}

func TestText(t *testing.T) {
	text := Info{Version: "1.2.3"}.Text()
	for _, want := range []string{"Autonomix CLI 1.2.3", Organization, Created, LicenseName, SupportEmail, "No accounts and no telemetry", "Bubble Tea"} {
		if !strings.Contains(text, want) {
			t.Errorf("Text() is missing %q", want)
		}
	}
}
