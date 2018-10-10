package main

import (
  "fmt"
  "os"
  "os/exec"
  "strings"
	"syscall"

  "github.com/aws/aws-sdk-go/aws"
  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
  "github.com/urfave/cli"
)

func main() {
  app := cli.NewApp()
  app.Name = "env-ssm"
  app.Usage = "run a command in an environment derived from the SSM parameter store"
  app.UsageText = "env-ssm [GLOBAL_OPTIONS] COMMAND [ARG ...]"
  app.Flags = []cli.Flag{
    cli.StringFlag{
      Name:  "prefix, p",
      Usage: "path prefix for parameter retrieval",
    },
    cli.BoolFlag{
      Name:  "clear-env, c",
      Usage: "start with an empty environment",
    },
    cli.StringFlag{
      Name:  "region, r",
      Usage: "specify AWS region",
    },
  }
  app.Before = validateArgs
  app.Action = runCommandInEnv
  app.Run(os.Args)
}

func runCommandInEnv(c *cli.Context) error {
  svc, err := initSsmService(c.String("region"))
  if err != nil {
    return cli.NewExitError(fmt.Sprintf("Failed to initialize SSM client: %v", err), 3)
  }
  env, err := buildEnvironment(svc, c.String("prefix"), c.Bool("clear-env"))
  if err != nil {
    return cli.NewExitError(fmt.Sprintf("Failed to build environment: %v", err), 4)
  }
  args := c.Args()
  cmdName := args[0]
	cmdPath, err := exec.LookPath(cmdName)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Command %s not found: %v", cmdName, err), 5)
	}
	if err := syscall.Exec(cmdPath, args, env); err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to execute %s: %v", cmdPath, err), 5)
	}
	return nil
}

func initSsmService(region string) (*ssm.SSM, error) {
  var options session.Options
  if len(region) == 0 {
    options = session.Options{
      SharedConfigState: session.SharedConfigEnable,
    }
  } else {
    options = session.Options{
      Config: aws.Config{Region: aws.String(region)},
    }
  }
  sess, err := session.NewSessionWithOptions(options)
  if err != nil {
    return nil, err
  }
  return ssm.New(sess), nil
}

func validateArgs(c *cli.Context) error {
  path := c.String("prefix")
  if len(path) == 0 {
    return cli.NewExitError("--prefix is required", 2)
  }
  if !strings.HasPrefix(path, "/") {
    return cli.NewExitError(fmt.Sprintf("Invalid prefix '%s', prefix should be an absolute path", path), 2)
  }
  if c.NArg() == 0 {
    return cli.NewExitError("No command given", 2)
  }
  return nil
}

func buildEnvironment(svc *ssm.SSM, path string, clear bool) ([]string, error) {
  var env []string
  if !clear {
    env = os.Environ()
  }
  if !strings.HasSuffix(path, "/") {
    path = path + "/"
  }
  params := new(ssm.GetParametersByPathInput)
  params.SetPath(path).SetRecursive(true).SetWithDecryption(true)
  err := svc.GetParametersByPathPages(
    params,
    func(page *ssm.GetParametersByPathOutput, lastPage bool) bool {
      for _, p := range page.Parameters {
        env = append(env, fmt.Sprintf("%s=%s", normalizeName(*p.Name, path), *p.Value))
      }
      return !lastPage
    })
  if err != nil {
    return nil, err
  }
  return env, nil
}

func normalizeName(n string, prefix string) string {
  n = strings.TrimPrefix(n, prefix)
  n = strings.Replace(n, "/", "_", -1)
	n = strings.Replace(n, "-", "_", -1)
  return strings.ToUpper(n)
}
