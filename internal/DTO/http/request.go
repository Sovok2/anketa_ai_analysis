package http

type questionData []struct {
	QuestionText string `json:"question_text"`
	Answer       string `json:"answer"`
	Time         string `json:"time"`
}

type Request struct {
	Type    string       `json:"type"`
	Answers questionData `json:"answers"`
}
