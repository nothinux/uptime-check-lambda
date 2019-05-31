package main

import (
	"log"
	"net/http"
	"net/http/httptrace"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
)

// track request and implement hook to report httptracing events
type transport struct {
	current *http.Request
}

// keep track of the current request
func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.current = req
	return http.DefaultTransport.RoundTrip(req)
}

type Host struct {
	Url string `json:"url"`
}

// for json output
type Response struct {
	Status       int           `json:"status:"`
	ResponseTime time.Duration `json:"responsetime:"`
}

func getResponseTime(url string) time.Duration {
	t := &transport{}
	req, _ := http.NewRequest("GET", url, nil)

	var start time.Time
	trace := &httptrace.ClientTrace{}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	client := &http.Client{
		Transport: t,
		Timeout:   2 * time.Second,
	}
	start = time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	return time.Since(start)

}

func getStatusCode(url string) int {
	req, _ := http.NewRequest("GET", url, nil)

	client := &http.Client{
		Timeout: 2 * time.Second,
	}
	resp, _ := client.Do(req)
	defer resp.Body.Close()

	return resp.StatusCode
}

func putMetrics(status int, rt time.Duration) {
	sess := session.Must(session.NewSession())

	svc := cloudwatch.New(sess)

	var host Host

	_, err := svc.PutMetricData(&cloudwatch.PutMetricDataInput{
		Namespace: aws.String("vmtest"),
		MetricData: []*cloudwatch.MetricDatum{
			&cloudwatch.MetricDatum{
				MetricName: aws.String("status"),
				Unit:       aws.String("Count"),
				Value:      aws.Float64(float64(status)),
				Dimensions: []*cloudwatch.Dimension{
					&cloudwatch.Dimension{
						Name:  aws.String("Site"),
						Value: aws.String(host.Url),
					},
				},
			},
			&cloudwatch.MetricDatum{
				MetricName: aws.String("responsetime"),
				Unit:       aws.String("Milliseconds"),
				Value:      aws.Float64(float64(rt)),
				Dimensions: []*cloudwatch.Dimension{
					&cloudwatch.Dimension{
						Name:  aws.String("Site"),
						Value: aws.String(host.Url),
					},
				},
			},
		},
	})

	if err != nil {
		// handle error
	}
}

func Uptime(host Host) (Response, error) {
	statuscode := getStatusCode(host.Url)
	responsetime := getResponseTime(host.Url)

	// send metrics to cloudwatch
	putMetrics(statuscode, responsetime)

	return Response{Status: statuscode, ResponseTime: responsetime}, nil
}

func main() {
	lambda.Start(Uptime)
}
