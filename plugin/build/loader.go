package build

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-alive/go-micro/plugin"
	"golang.org/x/tools/go/packages"
)

func buildSo(soPath string, parts []string) error {
	// check if .so file exists
	if _, err := os.Stat(soPath); os.IsExist(err) {
		return nil
	}

	path := filepath.Join(append([]string{"github.com/go-alive/go-plugins"}, parts...)...)
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName}, path)
	if err != nil {
		return err
	}

	// name and things
	name := pkgs[0].Name
	// type of plugin
	typ := parts[0]
	// new func signature
	newfn := fmt.Sprintf("New%s", strings.Title(typ))

	// micro has NewPlugin type def
	if typ == "micro" {
		newfn = "NewPlugin"
	}

	// now build the plugin
	if err := plugin.Build(soPath, &plugin.Config{
		Name:    name,
		Type:    typ,
		Path:    path,
		NewFunc: newfn,
	}); err != nil {
		return fmt.Errorf("Failed to build plugin %s: %v", name, err)
	}

	return nil
}

func load(p string) error {
	p = strings.TrimSpace(p)

	if len(p) == 0 {
		return nil
	}

	parts := strings.Split(p, "/")

	// 1 part means local plugin
	// plugin/foobar
	if len(parts) == 1 {
		return fmt.Errorf("Unknown plugin %s", p)
	}

	// set soPath to specified path
	soPath := p

	// build on the fly if not .so
	if !strings.HasSuffix(p, ".so") {
		// set new so path
		soPath = filepath.Join("plugin", p+".so")

		// build new .so
		if err := buildSo(soPath, parts); err != nil {
			return err
		}
	}

	// load the plugin
	pl, err := plugin.Load(soPath)
	if err != nil {
		return fmt.Errorf("Failed to load plugin %s: %v", soPath, err)
	}

	// Initialise the plugin
	return plugin.Init(pl)
}
