package cfg_test

import (
	"path/filepath"
	"testing"

	"github.com/lesomnus/xli/internal/x"
	"github.com/lesomnus/xli/xlitest"
)

func TestCmdConfig(t *testing.T) {
	pw := filepath.Join(t.TempDir(), "pw")
	writeAt(t, pw, "secret\n")
	p := write(t, `
ldap:
  bind: password
  key: ${file:`+pw+`}
  bases:
    a: dc=a
`)

	t.Run("prints the configuration with origins", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_INSECURE=true"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "--config", p, "config")
		x.NoError(res.Err)
		x.Equal(`ldap:
  insecure: true  # APP_LDAP_INSECURE
  bind: password  # `+p+`:3
  bases: {a: dc=a}  # `+p+`:6
  key: ${file:`+pw+`}  # `+p+`:4 via ${file:`+pw+`}
`, res.Stdout)
	}))
	t.Run("redacts literal secrets", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_KEY=hunter2"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "config")
		x.NoError(res.Err)
		x.Equal("ldap:\n  key: <redacted>  # APP_LDAP_KEY\n", res.Stdout)
		x.NotContains(res.Stdout, "hunter2")
	}))
	t.Run("lists the variables", x.F(func(x x.X) {
		root, _ := app(&AppConfig{}, env("APP_LDAP_BIND=key"), &ServeConfig{})
		res := xlitest.Run(x.T, root, "config", "env")
		x.NoError(res.Err)
		x.Equal("APP_LDAP_ADDR\nAPP_LDAP_INSECURE\nAPP_LDAP_BIND\nAPP_LDAP_BASES\nAPP_LDAP_TLS_CERT\nAPP_LDAP_TLS_KEY\nAPP_LDAP_KEY\n", res.Stdout)

		root, _ = app(&AppConfig{}, env("APP_LDAP_BIND=key"), &ServeConfig{})
		res = xlitest.Run(x.T, root, "config", "env", "--set")
		x.NoError(res.Err)
		x.Contains(res.Stdout, "APP_LDAP_ADDR\t-\n")
		x.Contains(res.Stdout, "APP_LDAP_BIND\tset\n")
	}))
}
