package ddns

import (
	"bytes"
	"fmt"
	"testing"
	"text/template"

	"github.com/stretchr/testify/require"
)

func TestName(t *testing.T) {
	tmpl := template.New("test")
	tt, err := tmpl.Parse("https://{{.username}}:{{.password}}")
	require.NoError(t, err)

	buf := bytes.NewBuffer(nil)

	m := make(map[string]string)
	m["username"] = "user"
	m["password"] = "pass"

	err = tt.Execute(buf, m)
	require.NoError(t, err)
	fmt.Println(buf)
}
