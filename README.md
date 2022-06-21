# AWS SDK latency

If you were debugging tail latency in AWS Go SDK,
you would probably try to trace the requests using
[httptrace](https://github.com/aws/aws-sdk-go/tree/main/example/aws/request/httptrace)
and realize that at least one second is spent at `Sign` step.

Jinli Liang from Rokt
[wrote a great explanation](https://www.rokt.com/engineering-blog/improving-app-latency-eks)
of what's going on.
In short, there are three issues:

- by default all AWS STS requests go to a single endpoint at `https://sts.amazonaws.com`.
  [AWS recommends](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_credentials_temp_enable-regions.html)
  using Regional AWS STS endpoints instead of the global endpoint
  to reduce latency, build in redundancy, and increase session token validity.
- increased latency from AWS STS request made by an SDK client during application startup
- increased latency when credentials expiry

This repository offers slightly refactored version of the code from the Rokt's post.

```go
package main

import (
	"context"
	"os"
	"os/signal"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/go-kit/log"
	"github.com/marselester/awscreds"
)

func main() {
	logger := log.NewJSONLogger(log.NewSyncWriter(os.Stderr))

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sess := session.Must(session.NewSession(&aws.Config{}))
	s3 := s3.New(sess)

	s, err := awscreds.NewSwapper(
		awscreds.WithLogger(logger),
	)
	if err != nil {
		logger.Log("msg", "failed to get aws credentials", "err", err)
		return
	}
	s.Attach(s3.Client)
	s.Run(ctx)
}
```
