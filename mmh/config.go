/*
 * Copyright 2018 mritd <mritd1234@gmail.com>
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package mmh

import (
	"errors"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"strconv"

	"path/filepath"

	"fmt"

	"bytes"
	"text/template"

	"sort"

	"github.com/mitchellh/go-homedir"
	"github.com/mritd/mmh/utils"
	"github.com/mritd/promptx"
)

var (
	MainCfg    MainConfig
	ContextCfg ContextConfig
	MaxProxy   int
)

// error def
var (
	inputEmptyErr    = errors.New("input is empty")
	inputTooLongErr  = errors.New("input length must be <= 12")
	serverExistErr   = errors.New("server name exist")
	notNumberErr     = errors.New("only number support")
	proxyNotFoundErr = errors.New("proxy server not found")
)

// find context by name
func (ctxList ContextList) FindContextByName(name string) (Context, bool) {
	for _, ctx := range ctxList.Context {
		if name == ctx.Name {
			return ctx, true
		}
	}
	return Context{}, false
}

// find server by name
func (servers Servers) FindServerByName(name string) *Server {

	for _, s := range servers {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// find servers by tag
func (servers Servers) FindServersByTag(tag string) Servers {

	ss := Servers{}

	for _, s := range servers {
		tmpServer := s
		for _, t := range tmpServer.Tags {
			if tag == t {
				ss = append(ss, tmpServer)
			}
		}
	}
	return ss
}

// add server
func AddServer() {

	// name
	p := promptx.NewDefaultPrompt(func(line []rune) error {
		if strings.TrimSpace(string(line)) == "" {
			return inputEmptyErr
		} else if len(line) > 12 {
			return inputTooLongErr
		}

		if s := ContextCfg.Servers.FindServerByName(string(line)); s != nil {
			return serverExistErr
		}
		return nil

	}, "Name:")

	name := p.Run()

	// tags
	p = promptx.NewDefaultPrompt(func(line []rune) error {
		// allow empty
		return nil
	}, "Tags:")

	// if it is a new tag, write the configuration file
	inputTags := strings.Split(p.Run(), ",")
	for _, tag := range inputTags {
		tagExist := false
		for _, extTag := range ContextCfg.Tags {
			if tag == extTag {
				tagExist = true
			}
		}
		if !tagExist {
			ContextCfg.Tags = append(ContextCfg.Tags, tag)
		}
	}

	// ssh user
	p = promptx.NewDefaultPrompt(func(line []rune) error {
		// allow empty
		return nil

	}, "User:")

	user := p.Run()
	if strings.TrimSpace(user) == "" {
		user = ContextCfg.Basic.User
	}

	// server address
	p = promptx.NewDefaultPrompt(func(line []rune) error {
		if strings.TrimSpace(string(line)) == "" {
			return inputEmptyErr
		}
		return nil

	}, "Address:")

	address := p.Run()

	// server port
	var port int
	p = promptx.NewDefaultPrompt(func(line []rune) error {
		if strings.TrimSpace(string(line)) != "" {
			if _, err := strconv.Atoi(string(line)); err != nil {
				return notNumberErr
			}
		}
		return nil

	}, "Port:")

	portStr := p.Run()
	if strings.TrimSpace(portStr) == "" {
		port = ContextCfg.Basic.Port
	} else {
		port, _ = strconv.Atoi(portStr)
	}

	// auth method
	var password, privateKey, privateKeyPassword string
	cfg := &promptx.SelectConfig{
		ActiveTpl:    "»  {{ . | cyan }}",
		InactiveTpl:  "  {{ . | white }}",
		SelectPrompt: "Auth Method",
		SelectedTpl:  "{{ \"» \" | green }}{{\"Method:\" | cyan }} {{ . | faint }}",
		DisPlaySize:  9,
		DetailsTpl: `
--------- SSH Auth Method ----------
{{ "Method:" | faint }}	{{ . }}`,
	}

	s := &promptx.Select{
		Items: []string{
			"PrivateKey",
			"Password",
		},
		Config: cfg,
	}

	idx := s.Run()

	// use private key
	if idx == 0 {
		p = promptx.NewDefaultPrompt(func(line []rune) error {
			// allow empty
			return nil

		}, "PrivateKey:")

		privateKey = p.Run()
		if strings.TrimSpace(privateKey) == "" {
			privateKey = ContextCfg.Basic.PrivateKey
		}

		p = promptx.NewDefaultPrompt(func(line []rune) error {
			// allow empty
			return nil

		}, "PrivateKey Password:")
		privateKeyPassword = p.Run()
		if strings.TrimSpace(privateKeyPassword) == "" {
			privateKeyPassword = ContextCfg.Basic.PrivateKeyPassword
		}
	} else {
		// use password
		p = promptx.NewDefaultPrompt(func(line []rune) error {
			// allow empty
			return nil

		}, "Password:")
		password = p.Run()
		if strings.TrimSpace(password) == "" {
			password = ContextCfg.Basic.Password
		}
	}

	// server proxy
	p = promptx.NewDefaultPrompt(func(line []rune) error {
		if strings.TrimSpace(string(line)) != "" {
			if ContextCfg.Servers.FindServerByName(string(line)) == nil {
				return proxyNotFoundErr
			}
		}
		return nil

	}, "Proxy:")

	proxy := p.Run()
	if strings.TrimSpace(proxy) == "" {
		proxy = ContextCfg.Basic.Proxy
	}

	// create server
	server := Server{
		Name:               name,
		Tags:               inputTags,
		User:               user,
		Address:            address,
		Port:               port,
		PrivateKey:         privateKey,
		PrivateKeyPassword: privateKeyPassword,
		Password:           password,
		Proxy:              proxy,
	}

	// Save
	ContextCfg.Servers = append(ContextCfg.Servers, &server)
	sort.Sort(ContextCfg.Servers)
	utils.CheckAndExit(ContextCfg.Write())
}

// delete server
func DeleteServer(serverNames []string) {

	var deletesIdx []int

	for _, serverName := range serverNames {
		for i, s := range ContextCfg.Servers {
			matched, err := filepath.Match(serverName, s.Name)
			// server name may contain special characters
			if err != nil {
				// check equal
				if strings.ToLower(s.Name) == strings.ToLower(serverName) {
					deletesIdx = append(deletesIdx, i)
				}
			} else {
				if matched {
					deletesIdx = append(deletesIdx, i)
				}
			}

		}

	}

	if len(deletesIdx) == 0 {
		utils.Exit("server not found!", 1)
	}

	// sort and delete
	sort.Ints(deletesIdx)
	for i, del := range deletesIdx {
		ContextCfg.Servers = append(ContextCfg.Servers[:del-i], ContextCfg.Servers[del-i+1:]...)
	}

	// save config
	sort.Sort(ContextCfg.Servers)
	utils.CheckAndExit(ContextCfg.Write())

}

// list servers
func ListServers() {

	tpl := `Name          User          Tags          Address
-------------------------------------------------------------
{{range . }}{{ .Name | ListLayout }}  {{ .User | ListLayout }}  {{ .Tags | MergeTag | ListLayout }}  {{ .Address }}:{{ .Port }}
{{end}}`
	t := template.New("").Funcs(map[string]interface{}{
		"ListLayout": listLayout,
		"MergeTag":   mergeTags,
	})

	_, _ = t.Parse(tpl)
	var buf bytes.Buffer
	utils.CheckAndExit(t.Execute(&buf, ContextCfg.Servers))
	fmt.Println(buf.String())
}

// print single server detail
func ServerDetail(serverName string) {
	s := ContextCfg.Servers.FindServerByName(serverName)
	if s == nil {
		utils.Exit("server not found!", 1)
	}
	tpl := `Name: {{ .Name }}
User: {{ .User }}
Address: {{ .Address }}:{{ .Port }}
Tags: {{ .Tags | MergeTag }}
Proxy: {{ .Proxy }}`
	t := template.New("").Funcs(map[string]interface{}{"MergeTag": mergeTags})
	_, _ = t.Parse(tpl)
	var buf bytes.Buffer
	utils.CheckAndExit(t.Execute(&buf, s))
	fmt.Println(buf.String())
}

// list contexts
func ListContexts() {

	tpl := `  Name          Path
---------------------------------
{{ range . }}{{ if .CurrentContext }}» {{ .Name | ListLayout }}{{ else }}  {{ .Name | ListLayout }}{{ end }}  {{ .ConfigPath }}
{{ end }}`

	t := template.New("").Funcs(map[string]interface{}{
		"ListLayout": listLayout,
		"MergeTag":   mergeTags,
	})
	_, _ = t.Parse(tpl)

	var ctxList ContextDetails
	for _, c := range MainCfg.Contexts.Context {
		ctxList = append(ctxList, ContextDetail{
			Name:           c.Name,
			ConfigPath:     c.ConfigPath,
			CurrentContext: c.Name == MainCfg.Contexts.Current,
		})
	}
	sort.Sort(ctxList)
	var buf bytes.Buffer
	utils.CheckAndExit(t.Execute(&buf, ctxList))
	fmt.Println(buf.String())
}

// set current context
func ContextUse(ctxName string) {
	_, ok := MainCfg.Contexts.FindContextByName(ctxName)
	if !ok {
		utils.Exit(fmt.Sprintf("context [%s] not found", ctxName), 1)
	}
	MainCfg.Contexts.Current = ctxName
	utils.CheckAndExit(MainCfg.Write())
}

// print layout func
func listLayout(name string) string {
	if len(name) < 12 {
		return fmt.Sprintf("%-12s", name)
	} else {
		return fmt.Sprintf("%-12s", utils.ShortenString(name, 12))
	}
}

// merge tags
func mergeTags(tags []string) string {
	return strings.Join(tags, ",")
}

// update context latest use timestamp
func UpdateContextTimestamp(_ *cobra.Command, _ []string) {

	home, _ := homedir.Dir()
	pidFile := filepath.Join(home, ".mmh", ".pid")
	_ = os.Remove(pidFile)

	MainCfg.Contexts.TimeStamp = time.Now()
	utils.CheckAndExit(MainCfg.Write())
}

// update context latest use timestamp in background
func UpdateContextTimestampTask(_ *cobra.Command, _ []string) {

	// if context auto downgrade is open
	if !MainCfg.Contexts.TimeStamp.IsZero() && MainCfg.Contexts.TimeOut != 0 && MainCfg.Contexts.AutoDowngrade != "" {

		home, _ := homedir.Dir()
		pid := strconv.Itoa(os.Getpid())
		pidFile := filepath.Join(home, ".mmh", ".pid")

		if MainCfg.Contexts.TimeOut < 60*time.Second {
			MainCfg.Contexts.TimeOut = 60 * time.Second
		}

		go func() {
			for {
				select {
				case <-time.Tick(MainCfg.Contexts.TimeOut - 3*time.Second):

					if _, err := os.Stat(pidFile); os.IsNotExist(err) {
						// write current pid to pid file
						utils.CheckAndExit(ioutil.WriteFile(pidFile, []byte(pid), 0644))
					} else {
						p, err := ioutil.ReadFile(pidFile)
						if err != nil {
							fmt.Println(err)
						}

						// check pid
						if string(p) == pid {
							MainCfg.Contexts.TimeStamp = time.Now()
							if err := MainCfg.Write(); err != nil {
								fmt.Println(err)
							}
						}
					}

				}
			}
		}()

	}

}
