package api

import (
	"SCITEduTool/Application/manager"
	"SCITEduTool/Application/module"
	"net/http"
)

func Exam(w http.ResponseWriter, r *http.Request) {
	base, errMessage := SetupAPI(w, r, map[string]string{
		"access_token": "",
	})
	if errMessage.HasInfo {
		errMessage.OutMessage(w)
		return
	}
	accessToken := base.GetParameter("access_token")
	username, errMessage := manager.TokenUnit.Check(manager.Token{
		AccessToken: accessToken,
	})
	if errMessage.HasInfo {
		errMessage.OutMessage(w)
		return
	}

	exam, errMessage := module.ExamModule.Get(username)
	if errMessage.HasInfo {
		errMessage.OutMessage(w)
		return
	}

	base.OnObjectResult(struct {
		Code    int               `json:"code"`
		Message string            `json:"message"`
		Exam    []module.ExamItem `json:"exam"`
	}{
		Code:    200,
		Message: "success.",
		Exam:    exam.Object,
	})
}
