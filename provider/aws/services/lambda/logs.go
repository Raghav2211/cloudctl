package lambda

import (
	ctlaws "cloudctl/provider/aws"
	"cloudctl/viewer"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

// logsAPI is the minimal client capability this file needs, letting tests
// substitute a fake instead of a real *cloudwatchlogs.Client (ADR-0007).
type logsAPI interface {
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

type logEvent struct {
	timestamp *int64 // epoch millis, per the CloudWatch Logs API
	message   string
}

// functionLogs is `ctl aws lambda logs <name>`'s output: recent log events
// from the function's log group, whose name follows AWS's fixed
// /aws/lambda/<function-name> convention — no separate lookup needed.
type functionLogs struct {
	functionName string
	events       []logEvent
}

type functionLogsFetcher struct {
	client        logsAPI
	functionName  string
	since         time.Duration
	limit         int32
	filterPattern string
}

func (f functionLogsFetcher) Fetch(ctx context.Context) (*functionLogs, error) {
	logGroupName := "/aws/lambda/" + f.functionName
	startTime := time.Now().Add(-f.since).UnixMilli()

	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: &logGroupName,
		StartTime:    &startTime,
		Limit:        aws.Int32(f.limit),
	}
	if f.filterPattern != "" {
		input.FilterPattern = &f.filterPattern
	}

	out, err := f.client.FilterLogEvents(ctx, input)
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}

	events := make([]logEvent, 0, len(out.Events))
	for _, e := range out.Events {
		events = append(events, logEvent{timestamp: e.Timestamp, message: derefStr(e.Message)})
	}
	return &functionLogs{functionName: f.functionName, events: events}, nil
}

func functionLogsViewer(data *functionLogs, err error) viewer.Viewer {
	if err != nil {
		return ctlaws.ErrorView(err)
	}
	if len(data.events) == 0 {
		return viewer.NewPanel().
			SetTitle(fmt.Sprintf("Logs for %s", data.functionName)).
			SetBody("no log events found in the given window")
	}

	tv := viewer.NewTableViewer()
	tv.SetStyle(viewer.DefaultTableStyle())
	tv.SetTitle(fmt.Sprintf("Logs for %s", data.functionName))
	tv.AddHeader(viewer.Row{"Timestamp", "Message"})
	for _, e := range data.events {
		ts := "-"
		if e.timestamp != nil {
			ts = time.UnixMilli(*e.timestamp).UTC().Format(time.RFC3339)
		}
		tv.AddRow(viewer.Row{ts, strings.TrimRight(e.message, "\n")})
	}
	return tv
}
