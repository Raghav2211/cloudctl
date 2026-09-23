package ec2

import (
	"cloudctl/ai"
	"cloudctl/evidence"
	ctlaws "cloudctl/provider/aws"
	ctltime "cloudctl/time"
	"cloudctl/viewer"
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"golang.org/x/sync/errgroup"
)

// maxConcurrentCloudWatchCalls bounds how many GetMetricStatistics calls run at
// once, to stay well under CloudWatch's default per-account TPS limits.
const maxConcurrentCloudWatchCalls = 8

type instanceListFetcher struct {
	client ec2.DescribeInstancesAPIClient
	tz     *ctltime.Timezone
	filter InstanceListFilter
}

type instanceDefinitionFetcher struct {
	client *ec2.Client
	tz     *ctltime.Timezone
	id     *string
}

// getMetricStatisticsAPI is the minimal client capability
// fetchInstanceStatistics needs, letting tests substitute a fake instead of
// a real *cloudwatch.Client.
type getMetricStatisticsAPI interface {
	GetMetricStatistics(ctx context.Context, params *cloudwatch.GetMetricStatisticsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.GetMetricStatisticsOutput, error)
}

type statisticsFetcher struct {
	client           ec2.DescribeInstancesAPIClient
	cloudwatchClient getMetricStatisticsAPI
	tz               *ctltime.Timezone
}

// describeSecurityGroupsAPI is the minimal client capability sgExplainFetcher
// needs, letting tests substitute a fake instead of a real *ec2.Client.
type describeSecurityGroupsAPI interface {
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

type sgExplainFetcher struct {
	client describeSecurityGroupsAPI
	sgId   string
}

func (f instanceListFetcher) Fetch(ctx context.Context) (*instanceListOutput, error) {
	instances, err := fetchInstanceList(ctx, f.client, f.filter)
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(instances) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}
	instancesByState := make(map[string][]*instanceSummary)
	for _, o := range instances {
		state := string(o.State.Name)
		instancesByState[state] = append(instancesByState[state], newInstanceSummary(o, f.tz))
	}
	return &instanceListOutput{instancesByState: instancesByState}, nil
}

func (f instanceDefinitionFetcher) Fetch(ctx context.Context) (*instanceDefinition, error) {
	return fetchInstanceDefinition(ctx, f.id, f.tz, f.client)
}

// Fetch retrieves CPU statistics for every running instance. Concurrency is
// bounded via errgroup+semaphore and each goroutine writes to its own index in
// a pre-sized slice, rather than appending from goroutines, to avoid a data
// race on the shared result set.
func (f statisticsFetcher) Fetch(ctx context.Context) (*instanceStatisticsListOutput, error) {
	runningFilter := *NewInstanceFilter(WithInstanceStates([]string{"running"}))
	instances, err := fetchInstanceList(ctx, f.client, runningFilter)
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(instances) == 0 {
		return nil, ctlaws.NewErrorInfo(NoInstanceFound(), viewer.INFO, nil)
	}

	currentTime := time.Now()
	startTime := time.Date(currentTime.Year(), currentTime.Month(), currentTime.Day()-2, 0, 0, 0, 0, currentTime.Location())

	g, gCtx := errgroup.WithContext(ctx)
	sem := make(chan struct{}, maxConcurrentCloudWatchCalls)
	results := make([]instanceStatisticsOutput, len(instances))

	for i, instance := range instances {
		i, instanceId := i, *instance.InstanceId
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = *fetchInstanceStatistics(gCtx, instanceId, startTime, currentTime, f.cloudwatchClient)
			return nil
		})
	}
	_ = g.Wait() // per-instance CloudWatch errors are carried in each result's apiError field, not fatal to the batch

	return &instanceStatisticsListOutput{stats: results}, nil
}

func fetchInstanceList(ctx context.Context, client ec2.DescribeInstancesAPIClient, instanceListFilter InstanceListFilter) ([]types.Instance, error) {
	instances := []types.Instance{}
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{
		Filters: instanceListFilter.requestFilters(),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, reservation := range page.Reservations {
			for _, instance := range reservation.Instances {
				if instanceListFilter.applyCustomFilter(instance) {
					instances = append(instances, instance)
				}
			}
		}
	}
	return instances, nil
}

func fetchInstanceDefinition(ctx context.Context, instanceId *string, tz *ctltime.Timezone, client *ec2.Client) (*instanceDefinition, error) {
	definition := newInstanceDefinition()
	networkinterfaces := []*instanceNetworkinterface{}

	data, err := client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: []string{*instanceId}})
	if err != nil {
		return nil, ctlaws.AWSError(err)
	}
	if len(data.Reservations) > 1 {
		return nil, errors.New("multiple reservation found how it's possible")
	}
	if len(data.Reservations) == 0 || len(data.Reservations[len(data.Reservations)-1].Instances) == 0 {
		return nil, NoInstanceFound()
	}
	reservation := data.Reservations[len(data.Reservations)-1]
	if len(reservation.Instances) > 1 {
		return nil, errors.New("multiple instance found how it's possible")
	}

	instance := reservation.Instances[len(reservation.Instances)-1]
	definition.SetInstanceSummary(newInstanceSummary(instance, tz))
	definition.SetInstanceDetail(newInstanceDetail(instance, tz))

	wg := new(sync.WaitGroup)
	if instance.BlockDeviceMappings != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			volumesSummary := fetchInstanceVolumeSummary(ctx, instance.BlockDeviceMappings, client)
			definition.SetVolumeSummary(volumesSummary)
		}()
	}
	if instance.NetworkInterfaces != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ruleSummary := fetchIngressEgressRuleSummary(ctx, instance.NetworkInterfaces, client)
			definition.SetInstanceIngressEgressRuleSummary(ruleSummary)
		}()
		for _, eni := range instance.NetworkInterfaces {
			networkinterfaces = append(networkinterfaces, newInstanceNetworkSummary(eni))
		}
		definition.SetNetworkInterfaces(networkinterfaces)
	}
	wg.Wait()

	facts := instanceDefinitionEvidence(definition)
	definition.applyAISummary(ctx, ai.NewClientFromEnv(), facts)

	return definition, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error (ADR-010). Mirrors
// sgExplanation.applyAISummary and bucketDefinition.applyAISummary in the s3
// package.
func (def *instanceDefinition) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		def.SetAISummaryUnavailable("no evidence could be gathered")
		return
	}
	summary, err := viewer.WithSpinner("Generating AI summary...", func() (string, error) {
		return client.Summarize(ctx, facts)
	})
	if err != nil {
		def.SetAISummaryUnavailable(err.Error())
		return
	}
	def.SetAISummary(summary)
}

func fetchInstanceVolumeSummary(ctx context.Context, volumemappings []types.InstanceBlockDeviceMapping, client *ec2.Client) *instanceVolumeSummary {
	volumeIds := []string{}
	volumes := []*instanceVolume{}
	for _, b := range volumemappings {
		if b.Ebs != nil && b.Ebs.VolumeId != nil {
			volumeIds = append(volumeIds, *b.Ebs.VolumeId)
		}
	}
	data, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{VolumeIds: volumeIds})
	if err != nil {
		errorInfo := ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		return newInstanceVolumeSummary(volumes, errorInfo)
	}
	for _, volume := range data.Volumes {
		volumes = append(volumes, newInstanceVolume(volume))
	}
	return newInstanceVolumeSummary(volumes, nil)
}

func fetchIngressEgressRuleSummary(ctx context.Context, enis []types.InstanceNetworkInterface, client *ec2.Client) *instanceIngressEgressRuleSummary {
	securityGroupIds := []string{}
	for _, eni := range enis {
		for _, sg := range eni.Groups {
			if sg.GroupId != nil {
				securityGroupIds = append(securityGroupIds, *sg.GroupId)
			}
		}
	}
	data, err := client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: securityGroupIds})
	if err != nil {
		errorInfo := ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
		return &instanceIngressEgressRuleSummary{apiError: errorInfo}
	}

	ingressRules := []*ingressRule{}
	egressRules := []*egressRule{}
	for _, sg := range data.SecurityGroups {
		ingressRules = append(ingressRules, newSecurityIngressRules(*sg.GroupId, *sg.GroupName, *sg.Description, sg.IpPermissions)...)
		egressRules = append(egressRules, newSecurityEgressRules(*sg.GroupId, *sg.GroupName, *sg.Description, sg.IpPermissionsEgress)...)
	}

	return &instanceIngressEgressRuleSummary{ingressRules: ingressRules, egressRules: egressRules}
}

func fetchInstanceStatistics(ctx context.Context, instanceId string, startTime, currentTime time.Time, client getMetricStatisticsAPI) *instanceStatisticsOutput {
	statsInput := cloudwatch.GetMetricStatisticsInput{
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("InstanceId"),
				Value: aws.String(instanceId),
			},
		},
		MetricName: aws.String("CPUUtilization"),
		Namespace:  aws.String("AWS/EC2"),
		Period:     aws.Int32(300),
		Statistics: []cwtypes.Statistic{cwtypes.StatisticAverage, cwtypes.StatisticMaximum, cwtypes.StatisticMinimum},
		StartTime:  &startTime,
		EndTime:    &currentTime,
	}
	result, err := client.GetMetricStatistics(ctx, &statsInput)
	if err != nil {
		return &instanceStatisticsOutput{instanceId: &instanceId, apiError: ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)}
	}
	if len(result.Datapoints) == 0 {
		return &instanceStatisticsOutput{instanceId: &instanceId, apiError: ctlaws.NewErrorInfo(fmt.Errorf("no CPU datapoints returned for instance %s", instanceId), viewer.INFO, nil)}
	}

	min := math.MaxFloat64
	max := 0.0
	total := 0.0

	for _, metricStats := range result.Datapoints {
		if metricStats.Minimum != nil && *metricStats.Minimum < min {
			min = *metricStats.Minimum
		}
		if metricStats.Maximum != nil && *metricStats.Maximum > max {
			max = *metricStats.Maximum
		}
		if metricStats.Average != nil {
			total += *metricStats.Average
		}
	}
	avg := total / float64(len(result.Datapoints))

	status := CPU_LOW
	if avg > 45 && avg <= 75 {
		status = CPU_MODERATE
	}
	if avg > 75 {
		status = CPU_HIGH
	}

	return &instanceStatisticsOutput{
		instanceId: &instanceId,
		Average:    &avg,
		Maximum:    &max,
		Minimum:    &min,
		CPUStatus:  status,
	}
}

// Fetch retrieves a single security group's rules and narrates them via
// Summarize, mirroring the s3 package's bucketConfigurationFetcher.Fetch —
// AI narration is additive (ADR-010): a failed or unreachable Summarize
// call never fails Fetch itself, it only leaves aiSummaryUnavailable set.
func (f sgExplainFetcher) Fetch(ctx context.Context) (*sgExplanation, error) {
	data, err := f.client.DescribeSecurityGroups(ctx, &ec2.DescribeSecurityGroupsInput{GroupIds: []string{f.sgId}})
	if err != nil {
		return nil, ctlaws.NewErrorInfo(ctlaws.AWSError(err), viewer.ERROR, nil)
	}
	if len(data.SecurityGroups) == 0 {
		return nil, ctlaws.NewErrorInfo(NoSecurityGroupFound(f.sgId), viewer.INFO, nil)
	}

	sg := data.SecurityGroups[0]
	sgId := derefStr(sg.GroupId)
	sgName := derefStr(sg.GroupName)
	description := derefStr(sg.Description)

	ingress := newSecurityIngressRules(sgId, sgName, description, sg.IpPermissions)
	egress := newSecurityEgressRules(sgId, sgName, description, sg.IpPermissionsEgress)

	explanation := newSGExplanation(sgId, sgName, description, ingress, egress)

	facts := securityGroupEvidence(sgId, sgName, description, ingress, egress)
	explanation.applyAISummary(ctx, ai.NewClientFromEnv(), facts)

	return explanation, nil
}

// applyAISummary sets aiSummary or aiSummaryUnavailable from the given
// evidence, never returning an error (ADR-010). Mirrors
// bucketDefinition.applyAISummary in the s3 package.
func (e *sgExplanation) applyAISummary(ctx context.Context, client *ai.Client, facts []evidence.Evidence) {
	if len(facts) == 0 {
		e.SetAISummaryUnavailable("no evidence could be gathered")
		return
	}
	summary, err := viewer.WithSpinner("Generating AI summary...", func() (string, error) {
		return client.Summarize(ctx, facts)
	})
	if err != nil {
		e.SetAISummaryUnavailable(err.Error())
		return
	}
	e.SetAISummary(summary)
}
