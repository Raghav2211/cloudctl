package services

import (
	"cloudctl/provider/aws/cli/globals"
	rawsec2 "cloudctl/provider/aws/services/ec2"
)

type eC2ListCmd struct {
	InstanceType []string `name:"type" help:"Return instance list of specific type | values [running,stopped,terminated,shutting-down,all]" default:"all"`
}

type instanceDefinitionCmd struct {
	Id string `name:"name" arg:"required"`
}

type EC2Command struct {
	List               eC2ListCmd            `name:"ls" cmd:"" help:"List ec2 instances"`
	InstacneDefinition instanceDefinitionCmd `name:"def" cmd:"" help:"Get ec2 instance definition"`
}

func (cmd *eC2ListCmd) Run(globals *globals.CLIFlag) error {
	icmd := rawsec2.NewinstanceListCommandExecutor(globals, cmd.InstanceType)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil

}

func (cmd *instanceDefinitionCmd) Run(globals *globals.CLIFlag) error {
	icmd := rawsec2.NewinstanceDescribeCommandExecutor(globals, cmd.Id)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}
