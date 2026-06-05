package metrics

import "math/rand/v2"

type Service struct {
}

type MetricType int

const (
	PlainText MetricType = iota
	AES
)

type Metrics struct {
	Reqs []int  `json:"request_count"`
	Type string `json:"type"`
}

func NewService() *Service {
	return &Service{}
}

func toString(m MetricType) string {
	switch m {
	case PlainText:
		return "PLAIN_TEXT"
	case AES:
		return "AES"
	}

	return "UNKNOW"
}

func (s *Service) GetMetrics() []Metrics {

	return []Metrics{
		{generateMockData(), toString(PlainText)},
		{generateMockData(), toString(AES)},
	}
}

func generateMockData() []int {
	data := make([]int, 20)
	max := 1000
	min := 800
	for i := range len(data) {
		data[i] = rand.IntN(max-min) + min
	}

	return data
}
