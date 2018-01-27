package main

import (
  "flag"
  "fmt"
  "os"
  "os/exec"
  "strings"

  "github.com/aws/aws-sdk-go/aws/session"
  "github.com/aws/aws-sdk-go/service/ssm"
)

func main() {
  help := flag.Bool("help", false, "Dispaly usage message")
  prefix := flag.String("prefix", "", "Path prefix for parameter retrieval")
  clear := flag.Bool("clean-env", false, "Start with an empty environment")
  flag.Parse()
  if *help {
    usage(nil)
  }
  if err := validatePrefix(*prefix); err != nil {
    usage(err)
  }
  if flag.NArg() < 1 {
    usage(fmt.Errorf("No command was specified"))
  }
  env, err := buildEnv(*prefix, *clear)
  if err != nil {
    errExit(err)
  }
  cmdName := flag.Arg(0)
  argv := flag.Args()[1:]
  cmd := exec.Command(cmdName, argv...)
  cmd.Stdin = os.Stdin
  cmd.Stdout = os.Stdout
  cmd.Stderr = os.Stderr
  cmd.Env = env
  if err := cmd.Run(); err != nil {
    errExit(fmt.Errorf("Failed to run %s: %v", cmdName, err))
  }
  os.Exit(0)
}

func buildEnv(path string, clear bool) ([]string, error) {
  var env []string
  if ! clear {
    env = os.Environ()
  }
  if ! strings.HasSuffix(path, "/") {
    path = path + "/"
  }
  sess := session.Must(session.NewSessionWithOptions(session.Options{
    SharedConfigState: session.SharedConfigEnable,
  }))
  params := &ssm.GetParametersByPathInput{}
  params.SetPath(path).SetRecursive(true).SetWithDecryption(true)
  svc := ssm.New(sess)
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
  return strings.ToUpper(n)
}

func usage(err error) {
  fmt.Fprintf(os.Stderr, "Usage: %s --prefix /some/path [--clear] COMMAND [ARG ...]\n", os.Args[0])
  if err != nil {
    errExit(err)
  }
  os.Exit(0)
}

func errExit(err error) {
  fmt.Fprintln(os.Stderr, err.Error())
  os.Exit(2)
}

func validatePrefix(s string) error {
  if ! (len(s) > 1 && strings.HasPrefix(s, "/")) {
    return fmt.Errorf("Invalid path prefix '%s': must be an absolute path", s)
  }
  return nil
}
