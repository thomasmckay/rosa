/*
Copyright (c) 2021 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package instancetypes

import (
	"fmt"
	"os"
	"text/tabwriter"

	cmv1 "github.com/openshift-online/ocm-sdk-go/clustersmgmt/v1"
	"github.com/spf13/cobra"

	"github.com/openshift/rosa/pkg/arguments"
	"github.com/openshift/rosa/pkg/aws"
	"github.com/openshift/rosa/pkg/helper"
	"github.com/openshift/rosa/pkg/interactive"
	"github.com/openshift/rosa/pkg/interactive/confirm"
	"github.com/openshift/rosa/pkg/output"
	"github.com/openshift/rosa/pkg/rosa"
)

var args struct {
	availabilityZones []string
	hasQuota          bool
	region            string
	roleArn           string
	listAll           bool
}

var Cmd = &cobra.Command{
	Use:     "instance-types",
	Aliases: []string{"instancetypes"},
	Short:   "List Instance types",
	Long:    "List Instance types that are available for use with ROSA.",
	Example: `  # List all instance types
  rosa list instance-types --all`,
	Run: run,
}

func init() {
	flags := Cmd.Flags()

	flags.StringSliceVar(
		&args.availabilityZones,
		"availability-zones",
		nil,
		"Limit listing to specified availability zones. "+
			"Format should be a comma-separated list.",
	)

	flags.BoolVar(
		&args.hasQuota,
		"has-quota",
		true,
		"Limit listing to only those with available quota for cluster creation.",
	)

	/*
		flags.StringVar(
			&args.region,
			"region",
			"",
			"Limit listing to a region.",
		)
	*/

	flags.StringVar(
		&args.roleArn,
		"role-arn",
		"",
		"STS Role ARN to use when listing instance types.",
	)

	flags.BoolVar(
		&args.listAll,
		"all",
		false,
		"List all directly from AWS regardless of availability for cluster creation. "+
			"(No other arguments accepted.)",
	)

	confirm.AddFlag(flags)
	interactive.AddFlag(flags)
	arguments.AddRegionFlag(flags)
	output.AddFlag(Cmd)
}

func run(cmd *cobra.Command, _ []string) {
	r := rosa.NewRuntime().WithOCM()
	defer r.Cleanup()

	var availabilityZones []string
	var selectAvailabilityZones bool

	supportedRegions, err := r.OCMClient.GetDatabaseRegionList()
	if err != nil {
		r.Reporter.Errorf("Unable to retrieve supported regions: %v", err)
	}
	awsClient := aws.GetAWSClientForUserRegion(r.Reporter, r.Logger, supportedRegions, false)
	r.AWSClient = awsClient

	region, err := aws.GetRegion(arguments.GetRegion())
	if err != nil {
		r.Reporter.Errorf("Error getting region: %v", err)
		os.Exit(1)
	}

	regionList, _, err := r.OCMClient.GetRegionList(false, args.roleArn, "", "",
		awsClient, false, false)
	if err != nil {
		r.Reporter.Errorf(fmt.Sprintf("%s", err))
		os.Exit(1)
	}
	if region == "" {
		r.Reporter.Errorf("Expected a valid AWS region")
		os.Exit(1)
	}

	if interactive.Enabled() {
		region, err = interactive.GetOption(interactive.Input{
			Question: "AWS region",
			Help:     cmd.Flags().Lookup("region").Usage,
			Options:  regionList,
			Default:  region,
			Required: true,
		})
		if err != nil {
			r.Reporter.Errorf("Expected a valid AWS region: %s", err)
			os.Exit(1)
		}
	}

	isAvailabilityZonesSet := cmd.Flags().Changed("availability-zones")
	if isAvailabilityZonesSet {
		availabilityZones = args.availabilityZones
	}
	if interactive.Enabled() {
		selectAvailabilityZones, err = interactive.GetBool(interactive.Input{
			Question: "Select availability zones",
			Help:     cmd.Flags().Lookup("availability-zones").Usage,
			Default:  false,
			Required: false,
		})
		if err != nil {
			r.Reporter.Errorf("Expected a valid value for select-availability-zones: %s", err)
			os.Exit(1)
		}

		if selectAvailabilityZones {
			optionsAvailabilityZones, err := awsClient.DescribeAvailabilityZones()
			if err != nil {
				r.Reporter.Errorf("Failed to get the list of the availability zone: %s", err)
				os.Exit(1)
			}

			availabilityZones, err = interactive.GetMultipleOptions(interactive.Input{
				Question: "Availability zones",
				Help:     cmd.Flags().Lookup("availability-zones").Usage,
				Required: false,
				Options:  optionsAvailabilityZones,
				Validators: []interactive.Validator{
					interactive.AvailabilityZonesCountValidator(true),
				},
			})
			if err != nil {
				r.Reporter.Errorf("Expected valid availability zones: %s", err)
				os.Exit(1)
			}
		}
	}

	if isAvailabilityZonesSet || selectAvailabilityZones {
		regionAvailabilityZones, err := awsClient.DescribeAvailabilityZones()
		if err != nil {
			r.Reporter.Errorf("Failed to get the list of the availability zone: %s", err)
			os.Exit(1)
		}
		for _, az := range availabilityZones {
			if !helper.Contains(regionAvailabilityZones, az) {
				r.Reporter.Errorf("Expected a valid availability zone, "+
					"'%s' doesn't belong to region '%s' availability zones", az, awsClient.GetRegion())
				os.Exit(1)
			}
		}
	}

	r.Reporter.Debugf("Fetching instance types")
	machineTypes, err := r.OCMClient.GetAvailableMachineTypesInRegion(region, args.availabilityZones,
		args.roleArn, awsClient)

	//machineTypes, err := r.OCMClient.GetAvailableMachineTypes()
	if err != nil {
		r.Reporter.Errorf("Failed to fetch instance types: %v", err)
		os.Exit(1)
	}

	if output.HasFlag() {
		var instanceTypes []*cmv1.MachineType
		for _, machine := range machineTypes.Items {
			instanceTypes = append(instanceTypes, machine.MachineType)
		}
		err = output.Print(instanceTypes)
		if err != nil {
			r.Reporter.Errorf("%s", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if len(machineTypes.Items) == 0 {
		r.Reporter.Warnf("There are no machine types supported for your account. Contact Red Hat support.")
		os.Exit(1)
	}

	// Create the writer that will be used to print the tabulated results:
	writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(writer, "ID\tCATEGORY\tCPU_CORES\tMEMORY\t\n")

	for _, machine := range machineTypes.Items {
		if !machine.Available {
			continue
		}
		availableMachine := machine.MachineType
		fmt.Fprintf(writer,
			"%s\t%s\t%d\t%s\n",
			availableMachine.ID(), availableMachine.Category(), int(availableMachine.CPU().Value()),
			ByteCountIEC(int(availableMachine.Memory().Value()),
				availableMachine.Memory().Unit()),
		)
	}
	writer.Flush()
}

func ByteCountIEC(b int, uValue string) string {
	var unit int
	if uValue == "B" {
		unit = 1024
	}
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= int64(unit)
		exp++
	}
	return fmt.Sprintf("%.1f %ciB",
		float64(b)/float64(div), "KMGTPE"[exp])
}
