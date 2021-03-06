package module

import (
	"SCITEduTool/Application/manager"
	"SCITEduTool/Application/stdio"
	"SCITEduTool/Application/unit"
	"crypto/md5"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type achieveModule interface {
	ExtractPrepare(info ExtractTaskInfo) (TaskStatus, stdio.MessagedError)
	ExtractFinal(info ExtractTaskInfo) stdio.MessagedError
	ExtractLink(info ExtractTaskInfo, accessToken string) string
	Get(username string, year string, semester int) (manager.AchieveObject, stdio.MessagedError)
	Refresh(username string, year string, semester int, session string, info manager.UserInfo) (manager.AchieveObject, stdio.MessagedError)
}

type achieveModuleImpl struct{}

var AchieveModule achieveModule = achieveModuleImpl{}

type ExtractTaskInfo struct {
	Username    string
	TaskID      int
	Year        string
	Semester    int
	TargetTasks []SingleTaskInfo
}

type TaskStatus struct {
	TaskID  int              `json:"task_id"`
	Success []SingleTaskInfo `json:"success"`
	Warn    []WarnTaskInfo   `json:"warn"`
	Failed  []FailedTaskInfo `json:"failed"`
}

type SingleTaskInfo struct {
	Username string `json:"uid"`
	Name     string `json:"name"`
}

type WarnTaskInfo struct {
	Username     string `json:"uid"`
	Name         string `json:"name"`
	NameInternal string `json:"name_internal"`
}

type FailedTaskInfo struct {
	Username  string `json:"uid"`
	Name      string `json:"name"`
	ErrorInfo string `json:"error_info"`
}

func (achieveModuleImpl achieveModuleImpl) ExtractPrepare(info ExtractTaskInfo) (TaskStatus, stdio.MessagedError) {
	status := TaskStatus{
		TaskID:  info.TaskID,
		Success: make([]SingleTaskInfo, 0),
		Warn:    make([]WarnTaskInfo, 0),
		Failed:  make([]FailedTaskInfo, 0),
	}
	if status.TaskID < 0 {
		status.TaskID = int(time.Now().Unix() / 300)
	}
	extractPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		stdio.LogWarn("", "运行目录获取失败", err)
		return status, stdio.GetErrorMessage(-500, "请求处理失败")
	}
	extractPath += "/achieve/extract/" + strconv.Itoa(status.TaskID) + "/" + info.Username + "/prepare/"
	//IF DEBUG
	//	extractPath = strings.ReplaceAll(extractPath, "/", "\\")
	//ENDIF
	_, err = os.Stat(extractPath)
	if err != nil {
		_ = os.MkdirAll(extractPath, 0644)
	} else if info.TaskID < 0 {
		return status, stdio.GetErrorMessage(-500, "短时间内请勿多次创建导出任务")
	}
	for _, singleTask := range info.TargetTasks {
		if singleTask.Username == "" {
			status.Failed = append(status.Failed, FailedTaskInfo{
				Name:      singleTask.Name,
				Username:  singleTask.Username,
				ErrorInfo: "学号为空",
			})
			continue
		}
		data, errMessage := manager.AchieveManager.Get(singleTask.Username, info.Year, info.Semester)
		if errMessage.HasInfo {
			status.Failed = append(status.Failed, FailedTaskInfo{
				Name:      singleTask.Name,
				Username:  singleTask.Username,
				ErrorInfo: data.ErrorInfo,
			})
			continue
		}
		err := ioutil.WriteFile(extractPath+singleTask.Username+".xlsx", data.Data, 0644)
		if err != nil {
			status.Failed = append(status.Failed, FailedTaskInfo{
				Name:      singleTask.Name,
				Username:  singleTask.Username,
				ErrorInfo: data.ErrorInfo,
			})
		} else if data.Name != singleTask.Name {
			status.Warn = append(status.Warn, WarnTaskInfo{
				Username:     singleTask.Username,
				Name:         singleTask.Name,
				NameInternal: data.Name,
			})
		} else {
			status.Success = append(status.Success, singleTask)
		}
	}
	return status, stdio.GetEmptyErrorMessage()
}

func (achieveModuleImpl achieveModuleImpl) ExtractFinal(info ExtractTaskInfo) stdio.MessagedError {
	extractPath, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		stdio.LogWarn("", "运行目录获取失败", err)
		return stdio.GetErrorMessage(-500, "请求处理失败")
	}
	extractPath += "/achieve/extract/" + strconv.Itoa(info.TaskID) + "/" + info.Username + "/"
	//IF DEBUG
	//	extractPath = strings.ReplaceAll(extractPath, "/", "\\")
	//ENDIF
	_, err = os.Stat(extractPath)
	if err != nil {
		stdio.LogWarn("", "导出预备目录获取失败", err)
		return stdio.GetErrorMessage(-500, "请求处理失败")
	}

	unit.Zip(extractPath+"prepare", extractPath+"extract_"+strconv.Itoa(info.TaskID)+".zip.prepare")
	err = os.Rename(extractPath+"extract_"+strconv.Itoa(info.TaskID)+".zip.prepare", extractPath+"extract_"+strconv.Itoa(info.TaskID)+".zip")
	if err != nil {
		stdio.LogWarn("", "成绩单读取失败", err)
	}
	return stdio.GetEmptyErrorMessage()
}

func (achieveModuleImpl achieveModuleImpl) ExtractLink(info ExtractTaskInfo, accessToken string) string {
	appSecret := manager.SignManager.GetDefaultAppSecretByPlatform("web")
	arg := "access_token=" + accessToken + "&task_id=" +
		strconv.Itoa(info.TaskID) + "&ts=" + strconv.Itoa(info.TaskID*300)
	h := md5.New()
	h.Write([]byte(arg + appSecret))
	sign := hex.EncodeToString(h.Sum(nil))
	arg += "&sign=" + sign
	link := ""
	//IF DEBUG
	//	link = "http://localhost:8000/api/achieve/extract/download?"
	//ELSE IF
	link = "https://tool.eclass.sgpublic.xyz/api/achieve/extract/download?"
	//ENDIF
	return link + arg
}

func (achieveModuleImpl achieveModuleImpl) Get(username string, year string, semester int) (manager.AchieveObject,
	stdio.MessagedError) {
	session, _, errMessage := SessionModule.Get(username, "")
	if errMessage.HasInfo {
		return manager.AchieveObject{}, errMessage
	}
	info, errMessage := InfoModule.Get(username)
	if errMessage.HasInfo {
		return manager.AchieveObject{}, errMessage
	}
	tableContent, errMessage := AchieveModule.Refresh(username, year, semester, session, info)
	if errMessage.HasInfo {
		return manager.AchieveObject{}, errMessage
	} else {
		return tableContent, stdio.GetEmptyErrorMessage()
	}
}

func (achieveModuleImpl achieveModuleImpl) Refresh(username string, year string, semester int, session string,
	info manager.UserInfo) (manager.AchieveObject,
	stdio.MessagedError) {
	switch info.Identify {
	case 0:
		return studentAchieve(username, year, semester, session, info)
	case 1:
		return teacherAchieve()
	default:
		return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
}

func studentAchieve(username string, year string, semester int, session string,
	info manager.UserInfo) (manager.AchieveObject,
	stdio.MessagedError) {
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	Button1 := "按学期查询"
	if year == "all" {
		Button1 = "在校学习成绩查询"
	} else if semester == 0 {
		Button1 = "按学年查询"
	}
	urlString := "http://218.6.163.93:8081/xscj.aspx?xh=" + username
	req, _ := http.NewRequest("GET", urlString, nil)
	req.AddCookie(&http.Cookie{Name: "ASP.NET_SessionId", Value: session})
	req.Header.Add("Referer", urlString)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := client.Do(req)
	if err != nil {
		stdio.LogError(username, "网络请求失败", err)
		return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
	body, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	r, _ := regexp.Compile("__VIEWSTATE\" value=\"(.*?)\"")
	viewState := r.FindString(string(body))
	if viewState == "" {
		stdio.LogError(username, "未发现 __VIEWSTATE", nil)
		return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}
	viewState = viewState[20 : len(viewState)-1]

	form := url.Values{}
	form.Set("__VIEWSTATE", viewState)
	form.Set("__VIEWSTATEGENERATOR", "17EB693E")
	form.Set("ddlXN", year)
	form.Set("ddlXQ", strconv.Itoa(semester))
	form.Set("txtQSCJ", "0")
	form.Set("txtZZCJ", "100")
	form.Set("Button1", Button1)
	req, _ = http.NewRequest("POST", urlString, strings.NewReader(strings.TrimSpace(form.Encode())))
	req.AddCookie(&http.Cookie{Name: "ASP.NET_SessionId", Value: session})
	req.Header.Add("Referer", urlString)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err = client.Do(req)
	if err != nil {
		stdio.LogError(username, "网络请求失败", err)
		return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}

	body, err = ioutil.ReadAll(resp.Body)
	resp.Body.Close()
	r, _ = regexp.Compile("__VIEWSTATE\" value=\"(.*?)\"")
	viewState = r.FindString(string(body))
	if viewState == "" {
		stdio.LogError(username, "未发现 __VIEWSTATE", nil)
		return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "请求处理出错")
	}

	bodyString := string(body)
	bodyString = strings.ReplaceAll(bodyString, "\n", "")
	bodyString = strings.ReplaceAll(bodyString, " class=\"alt\"", "")

	achieveObject := manager.AchieveObject{
		Current: []manager.CurrentAchieveItem{},
		Failed:  []manager.FailedAchieveItem{},
	}

	var currentMatch string
	var currentMatches []string
	var failedMatch string
	var failedMatches []string
	r, _ = regexp.Compile("id=\"DataGrid1\"(.*?)</table>")
	if !r.MatchString(bodyString) {
		stdio.LogInfo(username, "用户目标学期无成绩单")
		goto next
	}
	bodyString = strings.ReplaceAll(bodyString, "&nbsp;", "")
	currentMatch = r.FindString(bodyString)
	currentMatch = currentMatch[14 : len(currentMatch)-8]

	r, _ = regexp.Compile("<tr>(.*?)</tr>")
	currentMatches = r.FindAllString(currentMatch, -1)
	r, _ = regexp.Compile("<td>(.*?)</td>")
	for _, currentItem := range currentMatches {
		if !r.MatchString(currentItem) {
			continue
		}
		explodeGradeIndex := r.FindAllString(currentItem, -1)
		currentAchieveItem := manager.CurrentAchieveItem{}
		currentAchieveItem.Name = explodeGradeIndex[1][4 : len(explodeGradeIndex[1])-5]
		currentAchieveItem.PaperScore = explodeGradeIndex[3][4 : len(explodeGradeIndex[3])-5]
		currentAchieveItem.Mark = explodeGradeIndex[4][4 : len(explodeGradeIndex[4])-5]
		currentAchieveItem.Retake = explodeGradeIndex[6][4 : len(explodeGradeIndex[6])-5]
		currentAchieveItem.Rebuild = explodeGradeIndex[7][4 : len(explodeGradeIndex[7])-5]
		currentAchieveItem.Credit = explodeGradeIndex[8][4 : len(explodeGradeIndex[8])-5]
		achieveObject.Current = append(achieveObject.Current, currentAchieveItem)
	}

next:
	r, _ = regexp.Compile("id=\"Datagrid3\"(.*?)</table>")
	if !r.MatchString(bodyString) {
		stdio.LogInfo(username, "用户无挂科")
		goto result
	}
	failedMatch = r.FindString(bodyString)
	failedMatch = failedMatch[14 : len(failedMatch)-8]

	r, _ = regexp.Compile("<tr>(.*?)</tr>")
	failedMatches = r.FindAllString(failedMatch, -1)
	r, _ = regexp.Compile("<td>(.*?)</td>")
	for _, failedItem := range failedMatches {
		if !r.MatchString(failedItem) {
			continue
		}
		explodeGradeIndex := r.FindAllString(failedItem, -1)
		failedAchieveItem := manager.FailedAchieveItem{}
		failedAchieveItem.Name = explodeGradeIndex[1][4 : len(explodeGradeIndex[1])-5]
		failedAchieveItem.Mark = explodeGradeIndex[3][4 : len(explodeGradeIndex[3])-5]
		achieveObject.Failed = append(achieveObject.Failed, failedAchieveItem)
	}

result:
	manager.AchieveManager.Update(username, info, year, semester, achieveObject)
	return achieveObject, stdio.GetEmptyErrorMessage()
}

func teacherAchieve() (manager.AchieveObject, stdio.MessagedError) {
	return manager.AchieveObject{}, stdio.GetErrorMessage(-500, "什么？老师还有成绩单？(°Д°≡°Д°)")
}
