package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	tfe "github.com/hashicorp/go-tfe"
)

const (
	ENV_TERRAFORM_CLOUD_TOKEN        = "TERRAFORM_CLOUD_TOKEN"
	ENV_TERRAFORM_CLOUD_ORGANIZATION = "TERRAFORM_CLOUD_ORGANIZATION"
	ENV_TERRAFORM_CLOUD_WORKSPACE    = "TERRAFORM_CLOUD_WORKSPACE"
)

var organizationName string
var workspaceName string

func init() {
	flag.StringVar(&organizationName, "organization", "", "Terraform Cloud organization name")
	flag.StringVar(&workspaceName, "workspace", "", "Desired Terraform Cloud workspace name")
}

func main() {
	flag.Parse()

	if organizationName == "" {
		log.Println("No organization name provided as input argument, will fall back to environment variable")
		_, ok := os.LookupEnv(ENV_TERRAFORM_CLOUD_ORGANIZATION)
		if !ok {
			log.Fatalf("The organization name must be provided either as an input parameter or in the %s environment variable", ENV_TERRAFORM_CLOUD_ORGANIZATION)
		}
		organizationName = os.Getenv(ENV_TERRAFORM_CLOUD_ORGANIZATION)
		log.Println("Organization name read from environment variable")
	}

	if workspaceName == "" {
		log.Println("No workspace name provided as input argument, will fall back to environment variable")
		_, ok := os.LookupEnv(ENV_TERRAFORM_CLOUD_WORKSPACE)
		if !ok {
			log.Fatalf("A workspace name must be provided either as an input parameter or in the %s environment variable", ENV_TERRAFORM_CLOUD_WORKSPACE)
		}
		workspaceName = os.Getenv(ENV_TERRAFORM_CLOUD_WORKSPACE)
		log.Println("Workspace name read from environment variable")
	}

	token, ok := os.LookupEnv(ENV_TERRAFORM_CLOUD_TOKEN)
	if !ok || token == "" {
		log.Fatalf("%s environment variable must be set with a valid token", ENV_TERRAFORM_CLOUD_TOKEN)
	}

	config := &tfe.Config{
		Token:             token,
		RetryServerErrors: true,
	}

	client, err := tfe.NewClient(config)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()

	workspace, err := client.Workspaces.Read(ctx, organizationName, workspaceName)
	if err != nil {
		log.Fatal(err)
	}

	// run destroy plan and apply
	run, err := client.Runs.Create(ctx, tfe.RunCreateOptions{
		Message:   tfe.String("Automatically started via GitHub Actions"),
		AutoApply: tfe.Bool(true),
		IsDestroy: tfe.Bool(true),
		Workspace: workspace,
	})
	if err != nil {
		log.Fatal(err)
	}

	var numberOfIterations int = 0
	var sleepTimeBetweenIterations = 10
	var maxIterations int = 360
	for {
		r, err := client.Runs.Read(ctx, run.ID)
		if err != nil {
			log.Fatal(err)
		}

		if r.Status == tfe.RunApplied {
			log.Println("Destroy plan finished!")
			break
		}

		if r.Status == tfe.RunErrored {
			log.Fatalf("Could not destroy environment, check Terraform Cloud for status: %v", err)
		}

		log.Printf("Destroying ... (%d s)", sleepTimeBetweenIterations*numberOfIterations)
		time.Sleep(time.Duration(sleepTimeBetweenIterations) * time.Second)

		numberOfIterations += 1
		if numberOfIterations > maxIterations {
			log.Fatal("Destroy plan took more time than expected, please check status in Terraform Cloud")
		}
	}

	// delete workspace
	log.Println("Deleting workspace")
	err = client.Workspaces.Delete(ctx, organizationName, workspaceName)
	if err != nil {
		log.Fatal(err)
	}
}
