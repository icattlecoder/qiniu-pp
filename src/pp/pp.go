//author:
//	wangming
//	icattlecoder@gmail.com

package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/googollee/go-socket.io"
	"io/ioutil"
	"log"
	_ "mysql"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	REDMINEHOST         string `json:"redmine_host"`
	DOING_STATUS        []int  `json:"doing_status"`
	CODE_FINISHEDSTATUS []int  `json:"codefinished_status"`
	PUBLISHED_STATUS    []int  `json:"published_status"`
	LASTMONTHDATE       int    `json:"lastmonth_date"`
	CURRENTMONTHDATE    int    `json:"currentmonth_date"`
	APPKEY              string `json:"app_key"`
	PORT                string `json:"port"`
	DB_USERNAME         string `json:"db_username"`
	DB_PASSWORD         string `json:"db_password"`
	DB_NAME             string `json:"db_name"`
	DB_PORT             string `json:"db_port"`
}

func (p *pp) getRaw(url string) (body []byte, err error) {

	key := "key=" + p.config.APPKEY
	if strings.Contains(url, "?") {
		key = "&" + key
	} else {
		key = "?" + key
	}
	log.Println(p.config.REDMINEHOST + url + key)
	resp, err := http.Get(p.config.REDMINEHOST + url + key)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, err = ioutil.ReadAll(resp.Body)
	return
}

func (p *pp) get(url string, ret interface{}) (err error) {
	body, err := p.getRaw(url)
	err = json.Unmarshal(body, ret)
	return
}

func in(i int, src []int) bool {
	for _, v := range src {
		if v == i {
			return true
		}
	}
	return false
}

//====================================================
func (p *pp) getIssue(uid int) (is Issue_Comm, err error) {
	isw := IssuesWrap{Issue: is}
	err = p.get(fmt.Sprintf("/issues/%d.json", uid), &isw)
	is = isw.Issue
	return
}

func (p *pp) getIssueChangeSets(uid int)(author string,update_on string) {
	set := IssueChangeSet{}
	err := p.get(fmt.Sprintf("/issues/%d.json?include=journals", uid), &set)
	if err == nil {
		defer (func(){
			if in(set.Issues.Status.Id,p.config.PUBLISHED_STATUS){
				update_on = set.Issues.Updated_on
			}
		})()
		find:=func(notes string)string{
			rd := strings.NewReader(notes)
			scanner := bufio.NewScanner(rd)
			for scanner.Scan(){
				l := scanner.Text()
				if strings.Contains(l, "* author:") {
					return l[9:]
				}
			}
			return ""
		}
		update_on = set.Issues.Updated_on
		// log.Println("...", set.Issues.Journals)
		for _, v := range set.Issues.Journals {
			update_on = v.Created_on
			for _, vv := range v.Details {
				if vv.Name == "status_id" {
					i, err := strconv.ParseInt(vv.New_value, 10, 32)
					if err == nil && (in(int(i), p.config.CODE_FINISHEDSTATUS) || in(int(i), p.config.PUBLISHED_STATUS)) {
						update_on = v.Created_on
						if author=find(v.Notes);author!=""{
							return
						}
					}
				}
			}
		}
		//again
		if author == "" {
			for _, v := range set.Issues.Journals {
				if author=find(v.Notes);author!=""{
					return
				}
			}
		}
	} else {
		log.Println(err)
	}
	return
}

//获取开发人员
//* autho:<name>
func (p *pp) getAuthor(uid int) (string,string) {
	return p.getIssueChangeSets(uid)
}

func TimeConv(t string) string {
	d, _ := time.Parse("2006-01-02T15:04:05Z", t)
	return fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d", d.Year(), d.Month(), d.Day(), d.Hour(), d.Minute(), d.Second())
}

// start="2013-11-01"
func (p *pp) getIssuses(start, end string) {

	limitdate := "%3E%3C" + fmt.Sprintf("%s|%s", start, end)

	{
		finished := Issues{}
		for _, v := range p.config.PUBLISHED_STATUS {
			cnt := 0
			iss := Issues{}
			url := fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, 0, limitdate)
			err := p.get(url, &iss)
			if err != nil {
				log.Println(err)
			} else {
				finished.Total_count += iss.Total_count
				finished.Issues = append(finished.Issues, iss.Issues...)
			}
			cnt = len(iss.Issues)
			for cnt < iss.Total_count {
				url = fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
				err = p.get(url, &iss)
				if err != nil {
					log.Println(err)
				} else {
					finished.Issues = append(finished.Issues, iss.Issues...)
					cnt += len(iss.Issues)
				}
			}
		}

		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			for _, v := range finished.Issues {
				isslog := IssueLog{}
				isslog.Issue_id = v.Id
				author,up := p.getAuthor(v.Id)
				if author!="" {
					isslog.Author = author
				}
				isslog.Update_on = TimeConv(up)
				isslog.Project_id = v.Project.Id
				isslog.Issue_Status = v.Status.Id
				isslog.Issue_subject = v.Subject
				bl.Upsert(isslog)
			}
		})()
	}

	{
		readyToPub := Issues{}
		for _, v := range p.config.CODE_FINISHEDSTATUS {
			cnt := 0
			iss := Issues{}
			url := fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
			err := p.get(url, &iss)
			if err != nil {
				log.Println(err)
			} else {
				readyToPub.Total_count += iss.Total_count
				readyToPub.Issues = append(readyToPub.Issues, iss.Issues...)
			}
			cnt = len(iss.Issues)
			for cnt < iss.Total_count {
				url = fmt.Sprintf("/issues.json?status_id=%d&offset=%d&limit=100&sort=updated_on&updated_on=%s", v, cnt, limitdate)
				err = p.get(url, &iss)
				if err != nil {
					log.Println(err)
				} else {
					readyToPub.Issues = append(readyToPub.Issues, iss.Issues...)
					cnt += len(iss.Issues)
				}
			}
		}
		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			for _, v := range readyToPub.Issues {
				isslog := IssueLog{}
				isslog.Issue_id = v.Id
				author,up := p.getAuthor(v.Id)
				if author!="" {
					isslog.Author = author
				}
				isslog.Update_on = TimeConv(up)
				isslog.Project_id = v.Project.Id
				isslog.Issue_Status = v.Status.Id
				isslog.Issue_subject = v.Subject
				bl.Upsert(isslog)
			}
		})()
	}
	return
}

func (p *pp) r(id int) string {
	var n string
	for _, v := range p.rawprojects.Projects {
		if v.Id == id && v.Parent.Id > 0 {
			return p.r(v.Parent.Id)
		} else if v.Id == id {
			n = v.Name
			p.Projects[id] = n
			break
		}
	}
	return n
}

func (p *pp) getTopProject(id int) string {
	if v, ok := p.Projects[id]; ok {
		return v
	}
	return p.r(id)
}

func (p *pp) getProjects() {
	ps := Projects{}
	err := p.get("/projects.json", &ps)

	if err != nil {
		log.Println(err)
	} else {
		p.rawprojects = ps
	}
}

//====================================================
type pp struct {
	config         *Config
	rawprojects    Projects
	Projects       map[int]string
	sio            *socketio.SocketIOServer
}

func (p *pp) getStartQueryTime() string {

	now := time.Now()
	year := now.Year()
	mon := now.Month()
	date := now.Day()
	lastyear := year
	if date < p.config.CURRENTMONTHDATE {
		mon -= 1
	}
	lastmon := mon - 1
	if mon == 1 {
		lastyear -= 1
		lastmon = 12
	}
	return fmt.Sprintf("%d-%02d-%02d", lastyear, lastmon, p.config.LASTMONTHDATE)

}

func (p *pp) _codeFinished(w http.ResponseWriter, r *http.Request) {
	start := p.getStartQueryTime()
	bl, err := p.GetBL()
	if err != nil {
		log.Println("Open DB err")
		return
	}
	defer bl.db.Close()
	iss, err := bl.GetNoPubIsses(p.config.CODE_FINISHEDSTATUS, p.config.PUBLISHED_STATUS, start)
	for k, v := range iss {
		iss[k].ProjectName = p.getTopProject(v.Project_id)
		iss[k].Update_on = iss[k].Update_on[0:10]
	}
	if err != nil {
		log.Println("get Issues from DB error:", err)
	}
	bytes, _ := json.Marshal(iss)
	w.Write(bytes)
}

func (p *pp) _published(w http.ResponseWriter, r *http.Request) {
	start := p.getStartQueryTime()
	bl, err := p.GetBL()
	if err != nil {
		log.Println("Open DB err")
		return
	}
	defer bl.db.Close()
	iss, err := bl.GetIssues(p.config.PUBLISHED_STATUS, start)
	for k, v := range iss {
		iss[k].ProjectName = p.getTopProject(v.Project_id)
		iss[k].Update_on = iss[k].Update_on[0:10]
	}
	if err != nil {
		log.Println("get Issues from DB error:", err)
	}
	bytes, _ := json.Marshal(iss)
	w.Write(bytes)
}

func (p *pp) Notice(msg string, iss interface{}) {
	body, err := json.Marshal(iss)
	if err == nil {
		p.sio.Broadcast(msg, string(body))
		log.Println("Broadcast:", msg, ",body:", string(body))
	}
}

func (p *pp) Listener(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
	}
	id := r.FormValue("id")
	status_id := r.FormValue("status_id")
	iid, err := strconv.ParseInt(id, 10, 32)
	if err != nil {
		log.Println(err)
		return
	}
	istatus_id, err := strconv.ParseInt(status_id, 10, 32)
	if err != nil {
		log.Println(err)
		return
	}
	if in(int(istatus_id), p.config.CODE_FINISHEDSTATUS) || in(int(istatus_id), p.config.PUBLISHED_STATUS) {
		go (func() {
			bl, err := p.GetBL()
			if err != nil {
				log.Println("Open DB err")
				return
			}
			defer bl.db.Close()
			is, err := p.getIssue(int(iid))
			if err != nil {
				log.Println(err)
				return
			}
			isslog := IssueLog{}

			author,up := p.getAuthor(int(iid))
			if author!="" {
				isslog.Author = author
			}
			isslog.Update_on = TimeConv(up)

			isslog.Issue_id = is.Id
			isslog.Issue_Status = is.Status.Id
			isslog.Issue_subject = is.Subject
			isslog.Project_id = is.Project.Id
			isslog.ProjectName = p.getTopProject(is.Project.Id)
			log.Println(isslog)
			err = bl.Upsert(isslog)
			if err == nil {
				isslog.Update_on = isslog.Update_on[0:10]
				if in(int(istatus_id), p.config.CODE_FINISHEDSTATUS) {
					p.Notice("ready", isslog)
				} else {
					p.Notice("finished", isslog)
				}
			}
		})()
	}
}

func (p *pp) init() (err error) {
	p.Projects = make(map[int]string)
	p.getProjects()
	return
}

func (p *pp) GetBL() (bl *BL, err error) {
	db, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(127.0.0.1:%s)/%s", p.config.DB_USERNAME, p.config.DB_PASSWORD, p.config.DB_PORT, p.config.DB_NAME))
	if err != nil {
		return
	}
	bl = NewBL(db)
	return
}

func New() (p pp) {
	body, err := ioutil.ReadFile("./pp.json")
	if err != nil {

		fmt.Println(`
pp.json:
{
    "redmine_host": "http://pm.qbox.me/redmine",
    "doing_status": [84],
    "read2pub_status": [80],
    "finished_status": [5],
    "lastmonth_date": 1,
    "currentmonth_date": 1
    "port":8080 
    "db_username":root 
    "db_pasword": ******
    "db_name": redminedb 
    "db_port": 3306 
}`)

	}
	config := Config{}
	err = json.Unmarshal(body, &config)
	if err != nil {
		fmt.Println("config file error")
		os.Exit(-1)
	}
	log.Println(config)
	p = pp{config: &config}
	return
}

func (p *pp) proxy(w http.ResponseWriter, r *http.Request) {
	log.Println(r.URL.Path)
	body, err := p.getRaw(r.URL.Path[6:])
	if err != nil {
		log.Println(err)
	}
	fmt.Fprintf(w, string(body))
}

func (p *pp) fresh(w http.ResponseWriter, r *http.Request) {

	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
		return
	}
	name := r.FormValue("name")
	start := r.FormValue("start")
	end := r.FormValue("end")
	if name == "wangming" && start != "" && end != "" {
		p.getIssuses(start, end)
		w.Write([]byte("async...\n"))
	} else {
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
	}
}

func (p *pp) createTable(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		log.Println(err)
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
		return
	}
	name := r.FormValue("name")
	if name == "wangming" {
		bl, err := p.GetBL()
		if err != nil {
			log.Println("Open DB err")
			return
		}
		defer bl.db.Close()
		err = bl.CreateTable()
		if err != nil {
			log.Println("createTable Error:", err)
			w.Write([]byte("createTable Error!"))
		}
	} else {
		w.WriteHeader(401)
		w.Write([]byte("no auth\n"))
	}
}

func staticDirHandler(mux *socketio.SocketIOServer, prefix string, staticDir string, flags int) {
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {

		file := staticDir
		if prefix=="/"&&r.URL.Path=="/"{
			file +="/index.html"
		}else{
			file += r.URL.Path[len(staticDir)+1:]
		}

		if (flags) == 0 {
			fi, err := os.Stat(file)
			if err != nil || fi.IsDir() {
				http.NotFound(w, r)
				return
			}
		}

		http.ServeFile(w, r, file)
	})
}

func (p *pp) Run() {

	sock_config := &socketio.Config{}
	sock_config.HeartbeatTimeout = 2
	sock_config.ClosingTimeout = 4

	p.sio = socketio.NewSocketIOServer(sock_config)

	p.sio.Of("/pol").On("news", news)
	p.sio.Of("/pol").On("ping", func(ns *socketio.NameSpace) {
		p.sio.In("/pol").Broadcast("pong", nil)
	})

	p.sio.HandleFunc("/issues/codefinished", func(w http.ResponseWriter, r *http.Request) { p._codeFinished(w, r) })
	p.sio.HandleFunc("/issues/published", func(w http.ResponseWriter, r *http.Request) { p._published(w, r) })
	p.sio.HandleFunc("/listener", func(w http.ResponseWriter, r *http.Request) { p.Listener(w, r) })
	p.sio.HandleFunc("/proxy/", func(w http.ResponseWriter, r *http.Request) { p.proxy(w, r) })
	p.sio.HandleFunc("/async/", func(w http.ResponseWriter, r *http.Request) { p.fresh(w, r) })
	p.sio.HandleFunc("/createTable/", func(w http.ResponseWriter, r *http.Request) { p.createTable(w, r) })
	p.sio.HandleFunc("/getProjects/", func(w http.ResponseWriter, r *http.Request) { p.getProjects() })
	p.sio.HandleFunc("/getIssuse/", func(w http.ResponseWriter, r *http.Request) { 
		qs:=r.URL.Query()
		id:=qs.Get("id")
		iid, _ := strconv.ParseInt(id, 10, 32)
		p.getIssueChangeSets(int(iid))
		})

	staticDirHandler(p.sio, "/", "static", 0)

	err := p.init()
	if err != nil {
		log.Fatal("server start failed")
	}
	log.Fatal(http.ListenAndServe(":"+p.config.PORT, p.sio))
}

func news(ns *socketio.NameSpace, title, body string, article_num int) {
}
func main() {
	p := New()
	p.Run()
}
