package services

import (
	"cloudctl/provider/aws/cli/globals"
	"cloudctl/provider/aws/services/ec2"
	"log"
)

type eC2ListCmd struct {
	InstanceStates    []string `name:"state" help:"Return instance list of specific state(s) | values [running,stopped,terminated,shutting-down]" default:""`
	InstanceTypes     []string `name:"type" help:"Return instance list of specific type(s)" default:""`
	AvailabilityZones []string `name:"az" help:"Return instance list of specific availability zone(s)" default:""`
	VpcIds            []string `name:"vpc" help:"Return instance list of specific vpcId(s)" default:""`
	SubnetIds         []string `name:"subnet" help:"Return instance list of specific subnet(s)" default:""`
	HasPublicIp       *bool    `name:"has-public-ip" help:"Return instance list which have public ip associate"`
}

type instanceDefinitionCmd struct {
	Id string `name:"name" arg:"required"`
}

type EC2Command struct {
	List               eC2ListCmd            `name:"ls" cmd:"" help:"List ec2 instances"`
	InstacneDefinition instanceDefinitionCmd `name:"def" cmd:"" help:"Get ec2 instance definition"`
}

func (cmd *eC2ListCmd) Run(globals *globals.CLIFlag) error {

	filters := []ec2.InstanceListFilterOptFunc{
		ec2.WithAvailabilityZone(cmd.AvailabilityZones),
		ec2.WithInstanceStates(cmd.InstanceStates),
		ec2.WithInstanceType(cmd.InstanceTypes),
		ec2.WithSubnetsIds(cmd.SubnetIds),
		ec2.WithVpcIds(cmd.VpcIds),
	}
	if cmd.HasPublicIp != nil {
		filters = append(filters, ec2.WithHasPublicIp())
	}
	filter := ec2.NewInstanceFilter(filters...)

	icmd := ec2.NewinstanceListCommandExecutor(globals, *filter)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil

}

func (cmd *instanceDefinitionCmd) Run(globals *globals.CLIFlag) error {
	log.Default().Println("get definition for :", cmd.Id)
	icmd := ec2.NewinstanceDescribeCommandExecutor(globals, cmd.Id)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}
