package aws

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws"
	"gopkg.in/ini.v1"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/defaults"
	"github.com/aws/aws-sdk-go/aws/session"
)

const (
	AWS_DEFAULT_PROFILE = "AWS_DEFAULT_PROFILE"
	AWS_DEFAULT_REGION  = "AWS_DEFAULT_REGION"
)

func NewSession(profile, region string, debug bool) (sess *session.Session, err error) {

	defaultConfig := defaults.Get().Config

	logLevel := aws.LogLevel(aws.LogOff)
	if debug {
		logLevel = aws.LogLevel(aws.LogDebugWithRequestRetries | aws.LogDebugWithRequestErrors)
	}

	// fetch profile from env `see:AWS_DEFAULT_PROFILE` if not provide via command
	p := getEnv(profile, AWS_DEFAULT_PROFILE)
	// fetch region from env `see:AWS_DEFAULT_REGION` if not provide via command
	r := getEnv(region, AWS_DEFAULT_REGION)

	if len(r) == 0 {
		prompt := &survey.Input{Message: "Enter region?"}
		survey.AskOne(prompt, &r, survey.WithValidator(survey.Required))
	}

	cred := newCred(&p)

	config := defaultConfig.WithCredentials(cred).WithRegion(r).WithLogLevel(*logLevel)

	sess, err = session.NewSessionWithOptions(session.Options{
		Config: *config,
	})
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
			Default: profiles[0],
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

func getEnv(value, key string) string {
	if len(value) == 0 {
		return os.Getenv(key)
	}
	return value
}
