package cli

import (
	"cloudctl/provider/aws/cli/globals"
	rawsec2 "cloudctl/provider/aws/services/ec2"
)

type eC2ListCmd struct {
	// Test string `name:"testStr" cmd:""`
}

type describeInstanceCmd struct {
	Id string `name:"name" arg:"required"`
}

type eC2Command struct {
	List             eC2ListCmd          `name:"ls" cmd:"" help:"ec2 lists"`
	DescribeInstance describeInstanceCmd `name:"desc" cmd:"" help:"Avaiable region list"`
}

func (cmd *eC2ListCmd) Run(globals *globals.CLIFlag) error {
	icmd := rawsec2.NewinstanceListCommandExecutor(globals)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil

}

func (cmd *describeInstanceCmd) Run(globals *globals.CLIFlag) error {
	icmd := rawsec2.NewinstanceDescribeCommandExecutor(globals, cmd.Id)
	err := icmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

// func (cmd *EC2Command.Command) Run(globals *globals.CLIFlags) error {

// }
