package controller

import (
	"encoding/json"
	"fmt"
	"github.com/MG-RAST/AWE/lib/conf"
	"github.com/MG-RAST/AWE/lib/core"
	e "github.com/MG-RAST/AWE/lib/errors"
	. "github.com/MG-RAST/AWE/lib/logger"
	"github.com/MG-RAST/AWE/lib/logger/event"
	"github.com/MG-RAST/AWE/lib/request"
	"github.com/MG-RAST/AWE/vendor/github.com/MG-RAST/golib/goweb"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	logo = "\n" +
		" +--------------+  +----+   +----+   +----+  +--------------+\n" +
		" |              |  |    |   |    |   |    |  |              |\n" +
		" |    +----+    |  |    |   |    |   |    |  |    +---------+\n" +
		" |    |    |    |  |    |   |    |   |    |  |    |          \n" +
		" |    +----+    |  |    |   |    |   |    |  |    +---------+\n" +
		" |              |  |    |   |    |   |    |  |              |\n" +
		" |    +----+    |  |    |   |    |   |    |  |    +---------+\n" +
		" |    |    |    |  |    \\---/    \\---/    |  |    |          \n" +
		" |    |    |    |  |                      |  |    +---------+\n" +
		" |    |    |    |   \\        /---\\       /   |              |\n" +
		" +----+    +----+     \\-----/     \\-----/    +--------------+\n"
)

func PrintLogo() {
	fmt.Println(logo)
	return
}

type Query struct {
	Li map[string][]string
}

func (q *Query) Has(key string) bool {
	if _, has := q.Li[key]; has {
		return true
	}
	return false
}

func (q *Query) Value(key string) string {
	return q.Li[key][0]
}

func (q *Query) List(key string) []string {
	return q.Li[key]
}

func (q *Query) All() map[string][]string {
	return q.Li
}

func (q *Query) Empty() bool {
	if len(q.Li) == 0 {
		return true
	}
	return false
}

func LogRequest(req *http.Request) {
	host, _, _ := net.SplitHostPort(req.RemoteAddr)
	//	prefix := fmt.Sprintf("%s [%s]", host, time.Now().Format(time.RFC1123))
	suffix := ""
	if _, auth := req.Header["Authorization"]; auth {
		suffix = "AUTH"
	}
	url := ""
	if req.URL.RawQuery != "" {
		url = fmt.Sprintf("%s %s?%s", req.Method, req.URL.Path, req.URL.RawQuery)
	} else {
		url = fmt.Sprintf("%s %s", req.Method, req.URL.Path)
	}
	Log.Info("access", host+" \""+url+suffix+"\"")
}

func RawDir(cx *goweb.Context) {
	LogRequest(cx.Request)
	http.ServeFile(cx.ResponseWriter, cx.Request, fmt.Sprintf("%s/%s", conf.DATA_PATH, cx.Request.URL.Path))
}

func SiteDir(cx *goweb.Context) {
	LogRequest(cx.Request)
	if cx.Request.URL.Path == "/" {
		http.ServeFile(cx.ResponseWriter, cx.Request, conf.SITE_PATH+"/main.html")
	} else {
		http.ServeFile(cx.ResponseWriter, cx.Request, conf.SITE_PATH+cx.Request.URL.Path)
	}
}

type resource struct {
	R             []string `json:"resources"`
	F             []string `json:"info_indexes"`
	U             string   `json:"url"`
	D             string   `json:"documentation"`
	Title         string   `json:"title"` // title to show in AWE monitor
	C             string   `json:"contact"`
	I             string   `json:"id"`
	T             string   `json:"type"`
	S             string   `json:"queue_status"`
	V             string   `json:"version"`
	Time          string   `json:"server_time"`
	GitCommitHash string   `json:"git_commit_hash"`
}

func ResourceDescription(cx *goweb.Context) {
	LogRequest(cx.Request)
	r := resource{
		R:             []string{},
		F:             core.JobInfoIndexes,
		U:             apiUrl(cx) + "/",
		D:             siteUrl(cx) + "/",
		Title:         conf.TITLE,
		C:             conf.ADMIN_EMAIL,
		I:             "AWE",
		T:             core.Service,
		S:             core.QMgr.QueueStatus(),
		V:             conf.VERSION,
		Time:          time.Now().String(),
		GitCommitHash: conf.GIT_COMMIT_HASH,
	}
	if core.Service == "server" {
		r.R = []string{"job", "work", "client", "queue", "awf", "event"}
	} else if core.Service == "proxy" {
		r.R = []string{"client", "work"}
	}

	cx.WriteResponse(r, 200)
	return
}

func EventDescription(cx *goweb.Context) {
	LogRequest(cx.Request)
	cx.RespondWithData(event.EventDiscription)
	return
}

func apiUrl(cx *goweb.Context) string {
	if conf.API_URL != "" {
		return conf.API_URL
	}
	return "http://" + cx.Request.Host
}

func siteUrl(cx *goweb.Context) string {
	if conf.SITE_URL != "" {
		return conf.SITE_URL
	} else if strings.Contains(cx.Request.Host, ":") {
		return fmt.Sprintf("http://%s:%d", strings.Split(cx.Request.Host, ":")[0], conf.SITE_PORT)
	}
	return "http://" + cx.Request.Host
}

// helper function for create & update
func ParseMultipartForm(r *http.Request) (params map[string]string, files core.FormFiles, err error) {
	params = make(map[string]string)
	files = make(core.FormFiles)

	reader, err := r.MultipartReader()
	if err != nil {
		return
	}
	for {
		if part, err := reader.NextPart(); err == nil {

			if part.FileName() == "" {
				buffer := make([]byte, 32*1024)
				n, err := part.Read(buffer)
				if n == 0 || err != nil {
					break
				}
				params[part.FormName()] = fmt.Sprintf("%s", buffer[0:n])
			} else {

				tmpPath := fmt.Sprintf("%s/temp/%d%d", conf.DATA_PATH, rand.Int(), rand.Int())
				files[part.FormName()] = core.FormFile{Name: part.FileName(), Path: tmpPath, Checksum: make(map[string]string)}
				if tmpFile, err := os.Create(tmpPath); err == nil {
					buffer := make([]byte, 32*1024)
					for {
						n, err := part.Read(buffer)
						if n == 0 || err != nil {
							break
						}
						tmpFile.Write(buffer[0:n])
					}
					tmpFile.Close()
				} else {
					return nil, nil, err
				}
			}
		} else if err.Error() != "EOF" {
			fmt.Println("err here")
			return nil, nil, err
		} else {
			break
		}
	}

	return
}

func RespondTokenInHeader(cx *goweb.Context, token string) {
	cx.ResponseWriter.Header().Set("Datatoken", token)
	cx.Respond(nil, http.StatusOK, nil, cx)
	return
}

func RespondPrivateEnvInHeader(cx *goweb.Context, Envs map[string]string) (err error) {
	env_stream, err := json.Marshal(Envs)
	if err != nil {
		return err
	}
	cx.ResponseWriter.Header().Set("Privateenv", string(env_stream[:]))
	cx.Respond(nil, http.StatusOK, nil, cx)
	return
}

func AdminAuthenticated(cx *goweb.Context) bool {
	user, err := request.Authenticate(cx.Request)
	if err != nil {
		if err.Error() == e.NoAuth || err.Error() == e.UnAuth {
			cx.RespondWithError(http.StatusUnauthorized)
		} else {
			request.AuthError(err, cx)
		}
		return false
	}
	if _, ok := conf.Admin_Users[user.Username]; !ok {
		msg := fmt.Sprintf("user %s has no admin right", user.Username)
		cx.RespondWithErrorMessage(msg, http.StatusBadRequest)
		return false
	}
	return true
}
