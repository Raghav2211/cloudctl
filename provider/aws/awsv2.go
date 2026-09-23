package aws

import (
	"cloudctl/provider/aws/cli/globals"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"golang.org/x/term"
	"gopkg.in/ini.v1"
)

const (
	DEFAULT_REGION = "eu-west-1"
)

var (
	env_access_key        = []string{"AWS_ACCESS_KEY_ID"}
	env_secret_access_key = []string{"AWS_SECRET_ACCESS_KEY"}
	env_session_token     = []string{"AWS_SESSION_TOKEN"}
	env_profile           = []string{"AWS_DEFAULT_PROFILE", "AWS_PROFILE"}
	env_region            = []string{"AWS_DEFAULT_REGION", "AWS_REGION"}
)

type Credential struct {
	AccessKey    string
	SecretKey    string
	SessionToken string
}

func (cc Credential) hasKeys() bool {
	return len(cc.AccessKey) > 0 && len(cc.SecretKey) > 0
}

type CredentialConfig struct {
	Profile    string
	region     string
	Credential Credential
	useEnv     bool
	debug      bool
}

func NewCredentialConfig(cf globals.AWSCLIFlag, debug bool) CredentialConfig {
	// Fills in --region/--profile from the last-used session when left
	// unset, and remembers whatever ends up in effect for next time — a
	// user shouldn't have to repeat these on every command in one terminal
	// session. Explicit flags always win and become the new saved default.
	cf.ResolveSessionDefaults()
	return CredentialConfig{cf.Profile, cf.Region, Credential{cf.AccessKey, cf.SecretKey, cf.SessionToken}, cf.UseEnv, debug}
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

// isInteractive reports whether stdin is a terminal we can safely prompt on.
func isInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

// debugLog prints credential-resolution internals only when --debug is set.
// Without this, every command printed several lines of "load from direct
// credential" / "no credential provided, fallback on profiles" / etc. noise
// on every invocation, regardless of --debug.
func debugLog(debug bool, format string, args ...any) {
	if debug {
		log.Printf(format, args...)
	}
}

// getRegion resolves the AWS region from the provided value or env vars,
// prompting interactively only when stdin is a terminal; otherwise it fails
// fast instead of blocking forever on a prompt nobody can answer.
func getRegion(region string) (string, error) {
	region = getEnv(region, env_region)
	if len(region) != 0 {
		return region, nil
	}
	if !isInteractive() {
		return "", errors.New("no region provided: set --region, AWS_REGION, or AWS_DEFAULT_REGION (no terminal available to prompt)")
	}
	prompt := &survey.Input{
		Message: "Enter region?",
		Default: DEFAULT_REGION, // default region if not entered
	}
	if err := survey.AskOne(prompt, &region, survey.WithValidator(survey.Required)); err != nil {
		return "", fmt.Errorf("no region entered: %w", err)
	}
	return region, nil
}

// load AWS Config from direct credential provided via command or from environment
func loadConfigFromKeys(credential Credential, region string, debug bool) (aws.Config, error) {
	debugLog(debug, "load from direct credential")

	// fetch accessKey from env `see:env_access_key` if not provide via command and `useEnv` is true
	accessKey := getEnv(credential.AccessKey, env_access_key)
	// fetch secretKey from env `see:env_secret_access_key` if not provide via command and `useEnv` is true
	secretKey := getEnv(credential.SecretKey, env_secret_access_key)
	// fetch sessionToken from env `see:env_session_token` if not provide via command and `useEnv` is true
	sessionToken := getEnv(credential.SessionToken, env_session_token)

	httpClient := http.NewBuildableClient().WithDialerOptions(func(d *net.Dialer) {
		d.KeepAlive = -1
		d.Timeout = time.Millisecond * 500 // This setting represents the maximum amount of time a dial waits for a connection to be created.
	})

	credOption := config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
		accessKey,
		secretKey,
		sessionToken,
	))

	httpClientOption := config.WithHTTPClient(httpClient)

	awsConfigOptions := []func(*config.LoadOptions) error{config.WithRegion(region), credOption, httpClientOption}

	if debug {
		awsConfigOptions = append(
			awsConfigOptions,
			config.WithClientLogMode(aws.LogRequestWithBody|aws.LogRequestEventMessage|aws.LogRequest|aws.LogRequestWithBody|aws.LogRetries|aws.LogSigning),
		)
	}
	return config.LoadDefaultConfig(context.Background(), awsConfigOptions...)
}

// NewSessionV2 resolves an AWS config from direct credentials, falling back
// to a named profile. It never blocks indefinitely: profile/region prompts
// only happen when stdin is a terminal, and every failure returns a normal
// error instead of exiting the process.
func NewSessionV2(credentialConfig CredentialConfig) (*aws.Config, error) {

	region, err := getRegion(credentialConfig.region)
	if err != nil {
		return nil, err
	}

	if credentialConfig.Credential.hasKeys() || credentialConfig.useEnv {
		cfg, err := loadConfigFromKeys(credentialConfig.Credential, region, credentialConfig.debug)
		if err != nil {
			debugLog(credentialConfig.debug, "failed to load direct credential, fallback to profile selection, %v", err)
		} else if _, credErr := cfg.Credentials.Retrieve(context.Background()); credErr != nil {
			debugLog(credentialConfig.debug, "failed to retrieve direct credential, fallback to profile selection, %v", credErr)
		} else {
			debugLog(credentialConfig.debug, "build config from direct credential")
			return &cfg, nil
		}
	}

	debugLog(credentialConfig.debug, "no credential provided, fallback on profiles")

	profile := getEnv(credentialConfig.Profile, env_profile)
	if len(profile) == 0 {
		profiles, err := fetchConfiguredProfiles()
		if err != nil {
			return nil, fmt.Errorf("no AWS profile configured: %w", err)
		}
		if len(profiles) == 0 {
			return nil, errors.New("no AWS profile configured: set --profile, AWS_PROFILE, or configure ~/.aws/credentials")
		}
		if !isInteractive() {
			return nil, fmt.Errorf("no profile selected: set --profile or AWS_PROFILE (no terminal available to prompt); available profiles: %v", profiles)
		}
		prompt := &survey.Select{
			Message: "Choose a Profile:",
			Options: profiles,
		}
		if err := survey.AskOne(prompt, &profile, survey.WithValidator(survey.Required)); err != nil {
			return nil, fmt.Errorf("no profile selected: %w", err)
		}
		debugLog(credentialConfig.debug, "choose profile %s \n", profile)
	}

	debugLog(credentialConfig.debug, "load profile from enviornment %s \n", profile)
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion(region), config.WithSharedConfigProfile(profile))
	if err != nil {
		return nil, fmt.Errorf("failed to load config for profile %q: %w", profile, err)
	}
	if _, err := cfg.Credentials.Retrieve(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load credentials for profile %q: %w", profile, err)
	}
	return &cfg, nil
}
