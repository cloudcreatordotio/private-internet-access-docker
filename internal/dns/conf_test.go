package dns

import (
	"fmt"
	"strings"
	"testing"

	"github.com/qdm12/golibs/logging"
	"github.com/qdm12/golibs/network/mocks"
	"github.com/qdm12/private-internet-access-docker/internal/constants"
	"github.com/qdm12/private-internet-access-docker/internal/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_generateUnboundConf(t *testing.T) {
	t.Parallel()
	settings := settings.DNS{
		Provider:          constants.Cloudflare,
		AllowedHostnames:  []string{"a"},
		PrivateAddresses:  []string{"9.9.9.9"},
		BlockMalicious:    true,
		BlockSurveillance: false,
		BlockAds:          false,
		Verbosity:         2,
		LogLevel:          3,
	}
	client := &mocks.Client{}
	client.On("GetContent", constants.MaliciousBlockListHostnamesURL).
		Return([]byte("b\na\nc"), 200, nil).Once()
	client.On("GetContent", constants.MaliciousBlockListIPsURL).
		Return([]byte("c\nd\n"), 200, nil).Once()
	emptyLogger, err := logging.NewEmptyLogging()
	require.NoError(t, err)
	lines, errs := generateUnboundConf(settings, client, emptyLogger)
	require.Len(t, errs, 0)
	client.AssertExpectations(t)
	expected := `
server:
  cache-max-ttl: 9000
  cache-min-ttl: 3600
  do-ip4: yes
  do-ip6: no
  harden-algo-downgrade: yes
  harden-below-nxdomain: yes
  harden-referral-path: yes
  hide-identity: yes
  hide-version: yes
  interface: 127.0.0.1
  key-cache-size: 16m
  key-cache-slabs: 4
  msg-cache-size: 4m
  msg-cache-slabs: 4
  num-threads: 1
  port: 53
  prefetch-key: yes
  prefetch: yes
  root-hints: "/etc/unbound/root.hints"
  rrset-cache-size: 4m
  rrset-cache-slabs: 4
  rrset-roundrobin: yes
  tls-cert-bundle: "/etc/ssl/certs/ca-certificates.crt"
  trust-anchor-file: "/etc/unbound/root.key"
  use-syslog: no
  username: "nonrootuser"
  val-log-level: 3
  verbosity: 2
  local-zone: "b" static
  local-zone: "c" static
  private-address: 9.9.9.9
  private-address: c
  private-address: d
forward-zone:
  forward-tls-upstream: yes
  name: "."
  forward-addr: 1.1.1.1@853#cloudflare-dns.com
  forward-addr: 1.0.0.1@853#cloudflare-dns.com`
	assert.Equal(t, expected, "\n"+strings.Join(lines, "\n"))
}

func Test_buildBlocked(t *testing.T) {
	t.Parallel()
	type blockParams struct {
		blocked   bool
		content   []byte
		clientErr error
	}
	tests := map[string]struct {
		malicious        blockParams
		ads              blockParams
		surveillance     blockParams
		allowedHostnames []string
		privateAddresses []string
		hostnamesLines   []string
		ipsLines         []string
		errsString       []string
	}{
		"none blocked": {},
		"all blocked without lists": {
			malicious: blockParams{
				blocked: true,
			},
			ads: blockParams{
				blocked: true,
			},
			surveillance: blockParams{
				blocked: true,
			},
		},
		"all blocked with lists": {
			malicious: blockParams{
				blocked: true,
				content: []byte("malicious"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("ads"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("surveillance"),
			},
			hostnamesLines: []string{
				"  local-zone: \"ads\" static",
				"  local-zone: \"malicious\" static",
				"  local-zone: \"surveillance\" static"},
			ipsLines: []string{
				"  private-address: ads",
				"  private-address: malicious",
				"  private-address: surveillance"},
		},
		"all blocked with allowed hostnames": {
			malicious: blockParams{
				blocked: true,
				content: []byte("malicious"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("ads"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("surveillance"),
			},
			allowedHostnames: []string{"ads"},
			hostnamesLines: []string{
				"  local-zone: \"malicious\" static",
				"  local-zone: \"surveillance\" static"},
			ipsLines: []string{
				"  private-address: ads",
				"  private-address: malicious",
				"  private-address: surveillance"},
		},
		"all blocked with private addresses": {
			malicious: blockParams{
				blocked: true,
				content: []byte("malicious"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("ads"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("surveillance"),
			},
			privateAddresses: []string{"ads", "192.100.1.5"},
			hostnamesLines: []string{
				"  local-zone: \"ads\" static",
				"  local-zone: \"malicious\" static",
				"  local-zone: \"surveillance\" static"},
			ipsLines: []string{
				"  private-address: 192.100.1.5",
				"  private-address: ads",
				"  private-address: malicious",
				"  private-address: surveillance"},
		},
		"all blocked with lists and one error": {
			malicious: blockParams{
				blocked: true,
				content: []byte("malicious"),
			},
			ads: blockParams{
				blocked:   true,
				content:   []byte("ads"),
				clientErr: fmt.Errorf("ads error"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("surveillance"),
			},
			hostnamesLines: []string{
				"  local-zone: \"malicious\" static",
				"  local-zone: \"surveillance\" static"},
			ipsLines: []string{
				"  private-address: malicious",
				"  private-address: surveillance"},
			errsString: []string{"ads error", "ads error"},
		},
		"all blocked with errors": {
			malicious: blockParams{
				blocked:   true,
				clientErr: fmt.Errorf("malicious"),
			},
			ads: blockParams{
				blocked:   true,
				clientErr: fmt.Errorf("ads"),
			},
			surveillance: blockParams{
				blocked:   true,
				clientErr: fmt.Errorf("surveillance"),
			},
			errsString: []string{"malicious", "malicious", "ads", "ads", "surveillance", "surveillance"},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &mocks.Client{}
			if tc.malicious.blocked {
				client.On("GetContent", constants.MaliciousBlockListHostnamesURL).
					Return(tc.malicious.content, 200, tc.malicious.clientErr).Once()
				client.On("GetContent", constants.MaliciousBlockListIPsURL).
					Return(tc.malicious.content, 200, tc.malicious.clientErr).Once()
			}
			if tc.ads.blocked {
				client.On("GetContent", constants.AdsBlockListHostnamesURL).
					Return(tc.ads.content, 200, tc.ads.clientErr).Once()
				client.On("GetContent", constants.AdsBlockListIPsURL).
					Return(tc.ads.content, 200, tc.ads.clientErr).Once()
			}
			if tc.surveillance.blocked {
				client.On("GetContent", constants.SurveillanceBlockListHostnamesURL).
					Return(tc.surveillance.content, 200, tc.surveillance.clientErr).Once()
				client.On("GetContent", constants.SurveillanceBlockListIPsURL).
					Return(tc.surveillance.content, 200, tc.surveillance.clientErr).Once()
			}
			hostnamesLines, ipsLines, errs := buildBlocked(client, tc.malicious.blocked, tc.ads.blocked, tc.surveillance.blocked,
				tc.allowedHostnames, tc.privateAddresses)
			var errsString []string
			for _, err := range errs {
				errsString = append(errsString, err.Error())
			}
			assert.ElementsMatch(t, tc.errsString, errsString)
			assert.ElementsMatch(t, tc.hostnamesLines, hostnamesLines)
			assert.ElementsMatch(t, tc.ipsLines, ipsLines)
			client.AssertExpectations(t)
		})
	}
}

func Test_getList(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		content   []byte
		status    int
		clientErr error
		results   []string
		err       error
	}{
		"no result":     {nil, 200, nil, nil, nil},
		"bad status":    {nil, 500, nil, nil, fmt.Errorf("HTTP status code is 500 and not 200")},
		"network error": {nil, 200, fmt.Errorf("error"), nil, fmt.Errorf("error")},
		"results":       {[]byte("a\nb\nc\n"), 200, nil, []string{"a", "b", "c"}, nil},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &mocks.Client{}
			client.On("GetContent", "irrelevant_url").Return(
				tc.content, tc.status, tc.clientErr,
			).Once()
			results, err := getList(client, "irrelevant_url")
			if tc.err != nil {
				require.Error(t, err)
				assert.Equal(t, tc.err.Error(), err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.results, results)
			client.AssertExpectations(t)
		})
	}
}

func Test_buildBlockedHostnames(t *testing.T) {
	t.Parallel()
	type blockParams struct {
		blocked   bool
		content   []byte
		clientErr error
	}
	tests := map[string]struct {
		malicious        blockParams
		ads              blockParams
		surveillance     blockParams
		allowedHostnames []string
		lines            []string
		errsString       []string
	}{
		"nothing blocked": {
			lines:      nil,
			errsString: nil,
		},
		"only malicious blocked": {
			malicious: blockParams{
				blocked:   true,
				content:   []byte("site_a\nsite_b"),
				clientErr: nil,
			},
			lines: []string{
				"  local-zone: \"site_a\" static",
				"  local-zone: \"site_b\" static"},
			errsString: nil,
		},
		"all blocked with some duplicates": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_c"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("site_c\nsite_a"),
			},
			lines: []string{
				"  local-zone: \"site_a\" static",
				"  local-zone: \"site_b\" static",
				"  local-zone: \"site_c\" static"},
			errsString: nil,
		},
		"all blocked with one errored": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_c"),
			},
			surveillance: blockParams{
				blocked:   true,
				clientErr: fmt.Errorf("surveillance error"),
			},
			lines: []string{
				"  local-zone: \"site_a\" static",
				"  local-zone: \"site_b\" static",
				"  local-zone: \"site_c\" static"},
			errsString: []string{"surveillance error"},
		},
		"blocked with allowed hostnames": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_c\nsite_d"),
			},
			allowedHostnames: []string{"site_b", "site_c"},
			lines: []string{
				"  local-zone: \"site_a\" static",
				"  local-zone: \"site_d\" static"},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &mocks.Client{}
			if tc.malicious.blocked {
				client.On("GetContent", constants.MaliciousBlockListHostnamesURL).
					Return(tc.malicious.content, 200, tc.malicious.clientErr).Once()
			}
			if tc.ads.blocked {
				client.On("GetContent", constants.AdsBlockListHostnamesURL).
					Return(tc.ads.content, 200, tc.ads.clientErr).Once()
			}
			if tc.surveillance.blocked {
				client.On("GetContent", constants.SurveillanceBlockListHostnamesURL).
					Return(tc.surveillance.content, 200, tc.surveillance.clientErr).Once()
			}
			lines, errs := buildBlockedHostnames(client,
				tc.malicious.blocked, tc.ads.blocked, tc.surveillance.blocked, tc.allowedHostnames)
			var errsString []string
			for _, err := range errs {
				errsString = append(errsString, err.Error())
			}
			assert.ElementsMatch(t, tc.errsString, errsString)
			assert.ElementsMatch(t, tc.lines, lines)
			client.AssertExpectations(t)
		})
	}
}

func Test_buildBlockedIPs(t *testing.T) {
	t.Parallel()
	type blockParams struct {
		blocked   bool
		content   []byte
		clientErr error
	}
	tests := map[string]struct {
		malicious        blockParams
		ads              blockParams
		surveillance     blockParams
		privateAddresses []string
		lines            []string
		errsString       []string
	}{
		"nothing blocked": {
			lines:      nil,
			errsString: nil,
		},
		"only malicious blocked": {
			malicious: blockParams{
				blocked:   true,
				content:   []byte("site_a\nsite_b"),
				clientErr: nil,
			},
			lines: []string{
				"  private-address: site_a",
				"  private-address: site_b"},
			errsString: nil,
		},
		"all blocked with some duplicates": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_c"),
			},
			surveillance: blockParams{
				blocked: true,
				content: []byte("site_c\nsite_a"),
			},
			lines: []string{
				"  private-address: site_a",
				"  private-address: site_b",
				"  private-address: site_c"},
			errsString: nil,
		},
		"all blocked with one errored": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_c"),
			},
			surveillance: blockParams{
				blocked:   true,
				clientErr: fmt.Errorf("surveillance error"),
			},
			lines: []string{
				"  private-address: site_a",
				"  private-address: site_b",
				"  private-address: site_c"},
			errsString: []string{"surveillance error"},
		},
		"blocked with private addresses": {
			malicious: blockParams{
				blocked: true,
				content: []byte("site_a\nsite_b"),
			},
			ads: blockParams{
				blocked: true,
				content: []byte("site_c"),
			},
			privateAddresses: []string{"site_c", "site_d"},
			lines: []string{
				"  private-address: site_a",
				"  private-address: site_b",
				"  private-address: site_c",
				"  private-address: site_d"},
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client := &mocks.Client{}
			if tc.malicious.blocked {
				client.On("GetContent", constants.MaliciousBlockListIPsURL).
					Return(tc.malicious.content, 200, tc.malicious.clientErr).Once()
			}
			if tc.ads.blocked {
				client.On("GetContent", constants.AdsBlockListIPsURL).
					Return(tc.ads.content, 200, tc.ads.clientErr).Once()
			}
			if tc.surveillance.blocked {
				client.On("GetContent", constants.SurveillanceBlockListIPsURL).
					Return(tc.surveillance.content, 200, tc.surveillance.clientErr).Once()
			}
			lines, errs := buildBlockedIPs(client,
				tc.malicious.blocked, tc.ads.blocked, tc.surveillance.blocked, tc.privateAddresses)
			var errsString []string
			for _, err := range errs {
				errsString = append(errsString, err.Error())
			}
			assert.ElementsMatch(t, tc.errsString, errsString)
			assert.ElementsMatch(t, tc.lines, lines)
			client.AssertExpectations(t)
		})
	}
}
