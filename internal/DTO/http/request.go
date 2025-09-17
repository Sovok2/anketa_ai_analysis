package http

type questionData []struct {
	QuestionText string `json:"question_text"`
	Answer       string `json:"answer"`
	Time         string `json:"time"`
}

type Request struct {
	Answers questionData `json:"answers"`
}
