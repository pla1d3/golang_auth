package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

// vk_token template json
type Vk_token struct {
	Access_token string
	Expires_in   int
	User_id      int
}

// vk_response template json
type Vk_response struct {
	Response []Vk_user
}

// vk_user template json
type Vk_user struct {
	Id                int
	Bdate             string
	Can_access_closed bool
	Is_closed         bool
	First_name        string
	Last_name         string
	Screen_name       string
	Photo_big         string
}

// ya_response template json
type Ya_token struct {
	Access_token  string
	Token_type    string
	Expires_in    int
	Refresh_token string
}

// ya_user template json
type Ya_user struct {
	ID                string
	First_name        string
	Last_name         string
	Display_name      string
	Real_name         string
	Default_avatar_id string
	Is_avatar_empty   bool
	Birthday          string
	Login             string
	Sex               string
	Client_id         string
}

// fb_token template json
type Fb_token struct {
	Access_token string
	Token_type   string
	Expires      int
}

// fb_user template json
type Fb_user struct {
	ID    string
	Name  string
	Email string
	Image string
	Link  string
}

// RegHandle params
func AuthHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	r.ParseForm()

	params := mux.Vars(r)
	setting := r.URL.RawQuery

	if params["soc"] == "vk" {
		data := getUserInfoVk(setting)
		userInfo := checkSocUser(strconv.Itoa(data.Response[0].Id), "vk")

		if userInfo == nil {
			userInfo = createSocUserVk(data)
		}

		b, _ := json.Marshal(userInfo)
		fmt.Fprint(w, string(b))
	}

	if params["soc"] == "ya" {
		data := getUserInfoYa(setting)

		if data.ID != "" {
			userInfo := checkSocUser(data.ID, "ya")

			if userInfo == nil {
				userInfo = createSocUserYa(data)
			}

			b, _ := json.Marshal(userInfo)
			fmt.Fprint(w, string(b))
		} else {
			fmt.Fprint(w, "fail")
		}
	}

	if params["soc"] == "fb" {
		data := getUserInfoFb(setting)
		userInfo := checkSocUser(data.ID, "fb")

		if userInfo == nil {
			userInfo = createSocUserFb(data)
		}

		b, _ := json.Marshal(userInfo)
		fmt.Fprint(w, string(b))
	}

}

// Tokens
func getUserInfoVk(setting string) Vk_response {
	resp, _ := http.Get("https://oauth.vk.com/access_token?" + setting)
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var data Vk_token
	json.Unmarshal(body, &data)

	resp, _ = http.Get("https://api.vk.com/method/users.get?" +
		"uids=" + strconv.Itoa(data.User_id) +
		"&fields=" + "uid,first_name,last_name,screen_name,bdate,photo_big" +
		"&access_token=" + data.Access_token +
		"&v=5.92")
	defer resp.Body.Close()

	body, _ = ioutil.ReadAll(resp.Body)
	var dataUser Vk_response
	json.Unmarshal(body, &dataUser)

	return dataUser
}

func getUserInfoYa(setting string) Ya_user {
	m, _ := url.ParseQuery(setting)
	resp, _ := http.PostForm("https://oauth.yandex.ru/token", m)
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var data Ya_token
	json.Unmarshal(body, &data)

	resp, _ = http.Get("https://login.yandex.ru/info?" +
		"format=json" +
		"&oauth_token=" + data.Access_token)
	defer resp.Body.Close()

	body, _ = ioutil.ReadAll(resp.Body)
	var dataUser Ya_user
	json.Unmarshal(body, &dataUser)

	return dataUser
}

func getUserInfoFb(setting string) Fb_user {
	resp, _ := http.Get("https://graph.facebook.com/oauth/access_token?" + setting)
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var data Fb_token
	json.Unmarshal(body, &data)

	resp, _ = http.Get(`https://graph.facebook.com/me?` +
		`access_token=` + data.Access_token +
		`&fields=id,name,email`)
	defer resp.Body.Close()

	body, _ = ioutil.ReadAll(resp.Body)
	var dataUser Fb_user
	json.Unmarshal(body, &dataUser)
	dataUser.Image = "https://graph.facebook.com/v2.8/" + dataUser.ID + "/picture?type=large"
	dataUser.Link = "profile.php?id=" + dataUser.ID

	return dataUser
}

// SQL
func checkSocUser(id string, soc string) []*User {
	db, _ := sql.Open("mysql", "...")
	rows, _ := db.Query("SELECT id, email, login, date, city, status, rank, image, vk_id, fb_id, name, ya_login, fb_name FROM u_users WHERE "+soc+"_id = ?", id)
	db.Close()

	checkUser := make([]*User, 0)
	for rows.Next() {
		u := new(User)
		_ = rows.Scan(&u.ID, &u.Email, &u.Login, &u.Date, &u.City, &u.Status, &u.Rank, &u.Image, &u.Vk_id, &u.Fb_id, &u.Name, &u.Ya_login, &u.Fb_name)
		checkUser = append(checkUser, u)
	}

	if len(checkUser) != 0 {
		return checkUser
	}

	return nil
}

func createSocUserVk(data Vk_response) []*User {
	var login string
	loc, _ := time.LoadLocation("Europe/Moscow")
	date := time.Now().In(loc).Format("2006-01-02 03:04:05")
	name := data.Response[0].First_name + " " + data.Response[0].Last_name
	ya_login := ""
	fb_name := ""
	email := ""

	if regexp.MustCompile(`id\d+`).MatchString(data.Response[0].Screen_name) {
		login = getLatName(data.Response[0].First_name + "_" + data.Response[0].Last_name)
	} else {
		login = data.Response[0].Screen_name
	}

	matchedLogin(&login)

	return getUserInfo("vk", strconv.Itoa(data.Response[0].Id), login, date, data.Response[0].Photo_big, name, ya_login, fb_name, email)
}

func createSocUserYa(data Ya_user) []*User {
	loc, _ := time.LoadLocation("Europe/Moscow")
	date := time.Now().In(loc).Format("2006-01-02 03:04:05")
	image := "https://avatars.mds.yandex.net/get-yapic/" + data.Default_avatar_id + "/islands-300"
	login := data.Login
	ya_login := data.Login
	fb_name := ""
	email := data.Login + "@yandex.ru"
	matchedLogin(&login)
	return getUserInfo("ya", data.ID, login, date, image, data.Real_name, ya_login, fb_name, email)
}

func createSocUserFb(data Fb_user) []*User {
	loc, _ := time.LoadLocation("Europe/Moscow")
	date := time.Now().In(loc).Format("2006-01-02 03:04:05")
	ya_login := ""
	fb_name := data.Name
	email := data.Email
	login := strings.Join(strings.Split(strings.ToLower(data.Name), " "), "_")
	matchedLogin(&login)
	return getUserInfo("fb", data.ID, login, date, data.Image, data.Name, ya_login, fb_name, email)
}

func getUserInfo(soc string, id string, login string, date string, image string, name string, ya_login string, fb_name string, email string) []*User {
	db, _ := sql.Open("mysql", "...")
	_, _ = db.Query(`INSERT INTO u_users (`+soc+`_id, login, date, image, name, ya_login, fb_name, email) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, id, login, date, image, name, ya_login, fb_name, email)

	rows, _ := db.Query("SELECT id, email, login, date, city, status, rank, image, vk_id, fb_id, name, ya_login, fb_name FROM u_users WHERE "+soc+"_id = ?", id)
	db.Close()

	user := make([]*User, 0)
	for rows.Next() {
		u := new(User)
		_ = rows.Scan(&u.ID, &u.Email, &u.Login, &u.Date, &u.City, &u.Status, &u.Rank, &u.Image, &u.Vk_id, &u.Fb_id, &u.Name, &u.Ya_login, &u.Fb_name)
		user = append(user, u)
	}

	return user
}

// helpers
func getLatName(name string) string {
	alphabet := map[string]string{
		"а": "a", "б": "b", "в": "v", "г": "g", "д": "d",
		"е": "e", "ё": "yo", "ж": "zh", "з": "z", "и": "i",
		"й": "j", "к": "k", "л": "l", "м": "m", "н": "n",
		"о": "o", "п": "p", "р": "r", "с": "s", "т": "t",
		"у": "u", "ф": "f", "х": "h", "ц": "c", "ч": "ch",
		"ш": "sh", "щ": "shh", "ъ": "", "ы": "y", "ь": "",
		"э": "e", "ю": "yu", "я": "ya",
	}

	newName := ""
	nameArr := strings.Split(strings.ToLower(name), "")

	for _, val := range nameArr {
		if alphabet[val] != "" {
			newName = newName + alphabet[val]
		} else {
			newName = newName + val
		}
	}

	return newName
}
