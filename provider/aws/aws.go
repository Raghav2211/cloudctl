package aws

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"gopkg.in/ini.v1"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	DEFAULT_REGION = "eu-west-1"
)

var (
	env_profile = []string{"AWS_DEFAULT_PROFILE", "AWS_PROFILE"}
	env_region  = []string{"AWS_DEFAULT_REGION", "AWS_REGION"}
)

type Client struct {
	EC2          *ec2.EC2
	S3           *s3.S3
	Cloudwatch   *cloudwatch.CloudWatch
	S3Downloader *s3manager.Downloader
}

func NewClient(profile, region string, debug bool) (client *Client) {
	session, _ := newSession(profile, region, debug)
	client = &Client{
		EC2:          ec2.New(session),
		S3:           s3.New(session),
		Cloudwatch:   cloudwatch.New(session),
		S3Downloader: s3manager.NewDownloader(session),
	}
	return
}

func newSession(profile, region string, debug bool) (sess *session.Session, err error) {

	defaultConfig := defaults.Get().Config

	logLevel := aws.LogLevel(aws.LogOff)
	if debug {
		logLevel = aws.LogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	}

	// fetch profile from env `see:env_profile` if not provide via command
	p := getEnv(profile, env_profile)
	// fetch region from env `see:env_region` if not provide via command
	r := getEnv(region, env_region)

	if len(r) == 0 {
		prompt := &survey.Input{
			Message: "Enter region?",
			Default: DEFAULT_REGION, // default region if not entered
		}
		survey.AskOne(prompt, &r, survey.WithValidator(survey.Required))
	}

	cred := newCred(&p)

	config := defaultConfig.WithCredentials(cred).WithRegion(r).WithLogLevel(*logLevel)

	session.NewSession(config)
	sess = session.Must(session.NewSession(config))
	return
}

func newCred(profile *string) (cred *credentials.Credentials) {
	credProviders := []credentials.Provider{}

	if len(*profile) != 0 {
		credProviders = append(credProviders, &credentials.SharedCredentialsProvider{
			Profile: *profile,
		})
	}
	credProviders = append(credProviders, &credentials.EnvProvider{})

	cred = credentials.NewChainCredentials(credProviders)
	credV, err := cred.Get()

	if err != nil || !credV.HasKeys() {
		profiles, profileErr := fetchConfiguredProfiles()
		if profileErr != nil {
			fmt.Println("no profile exists")
			return nil
		}
		var prompt = &survey.Select{
			Message: "Choose a Profile:",
			Options: profiles,
		}
		err := survey.AskOne(prompt, profile, survey.WithValidator(survey.Required))
		if err != nil {
			fmt.Println("no profile selected")
			return nil
		}
		cred = credentials.NewChainCredentials([]credentials.Provider{
			&credentials.SharedCredentialsProvider{
				Profile: *profile,
			},
		})
	}
	return
}

func fetchConfiguredProfiles() ([]string, error) {
	credFile := config.DefaultSharedCredentialsFilename()
	f, err := ini.Load(credFile)
	if err == nil {

		arr := []string{}
		for _, v := range f.Sections() {
			if len(v.Keys()) != 0 {
				arr = append(arr, v.Name())
			}
		}
		return arr, nil
	}
	return nil, err
}

func getEnv(value string, keys []string) string {
	if len(value) == 0 {
		for _, key := range keys {
			v := os.Getenv(key)
			if len(v) != 0 {
				return os.Getenv(key)
			}
		}
	}
	return value
}
