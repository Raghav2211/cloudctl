package ec2

import (
	"cloudctl/evidence"
	"fmt"
)

// securityGroupEvidence converts a security group's already-parsed rules
// into Fact-tagged evidence for Summarize.
func securityGroupEvidence(sgId, sgName, description string, ingress []*ingressRule, egress []*egressRule) []evidence.Evidence {
	facts := []evidence.Evidence{
		{Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "GroupName", Value: sgName, Confidence: evidence.Fact},
	}
	if description != "" {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "Description",
			Value: description, Confidence: evidence.Fact,
		})
	}
	for _, r := range ingress {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "IngressRule",
			Value:      fmt.Sprintf("%s/%s from %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
			Confidence: evidence.Fact,
		})
	}
	for _, r := range egress {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeSecurityGroups", ResourceID: sgId, Field: "EgressRule",
			Value:      fmt.Sprintf("%s/%s to %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
			Confidence: evidence.Fact,
		})
	}
	return facts
}

func derefStr(s *string) string {
	if s == nil {
		return NO_VALUE
	}
	return *s
}

// instanceDefinitionEvidence converts an already-fetched instanceDefinition's
// summary/detail/volumes/rules/network data into Fact-tagged evidence for
// Summarize, mirroring securityGroupEvidence and s3's
// bucketDefinitionEvidence. Every sub-fetch (volumes, rules) is fetched
// concurrently in fetchInstanceDefinition and may have failed independently,
// so each section only contributes evidence when its data is actually
// present, rather than assuming full population.
func instanceDefinitionEvidence(def *instanceDefinition) []evidence.Evidence {
	var facts []evidence.Evidence
	const source = "ec2:DescribeInstances"

	if s := def.summary; s != nil {
		facts = append(facts,
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "InstanceType", Value: derefStr(s.typee), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "State", Value: derefStr(s.state), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "AvailabilityZone", Value: derefStr(s.az), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "VpcId", Value: derefStr(s.vpcId), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "SubnetId", Value: derefStr(s.subnetId), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "PublicIp", Value: derefStr(s.publicIp), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "PrivateIp", Value: derefStr(s.privateIp), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: derefStr(s.id), Field: "IAMRoleARN", Value: derefStr(s.iamroleArn), Confidence: evidence.Fact},
		)
	}

	resourceID := NO_VALUE
	if def.summary != nil {
		resourceID = derefStr(def.summary.id)
	}

	if d := def.detail; d != nil {
		facts = append(facts,
			evidence.Evidence{Source: source, ResourceID: resourceID, Field: "Platform", Value: derefStr(d.platform), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: resourceID, Field: "AmiId", Value: derefStr(d.amiId), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: resourceID, Field: "Monitoring", Value: derefStr(d.monitor), Confidence: evidence.Fact},
			evidence.Evidence{Source: source, ResourceID: resourceID, Field: "OSDetails", Value: derefStr(d.osdetails), Confidence: evidence.Fact},
		)
	}

	if vs := def.volumesSummary; vs != nil && vs.apiError == nil {
		for _, volume := range vs.volumes {
			encrypted := "false"
			if volume.isEncrypt != nil && *volume.isEncrypt {
				encrypted = "true"
			}
			size := NO_VALUE
			if volume.size != nil {
				size = fmt.Sprintf("%d", *volume.size)
			}
			facts = append(facts, evidence.Evidence{
				Source: "ec2:DescribeVolumes", ResourceID: resourceID, Field: "Volume",
				Value:      fmt.Sprintf("size=%sGiB encrypted=%s state=%s", size, encrypted, derefStr(volume.state)),
				Confidence: evidence.Fact,
			})
		}
	}

	if rs := def.ruleSummary; rs != nil && rs.apiError == nil {
		for _, r := range rs.ingressRules {
			facts = append(facts, evidence.Evidence{
				Source: "ec2:DescribeSecurityGroups", ResourceID: resourceID, Field: "IngressRule",
				Value:      fmt.Sprintf("%s/%s from %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
				Confidence: evidence.Fact,
			})
		}
		for _, r := range rs.egressRules {
			facts = append(facts, evidence.Evidence{
				Source: "ec2:DescribeSecurityGroups", ResourceID: resourceID, Field: "EgressRule",
				Value:      fmt.Sprintf("%s/%s to %s (%s)", derefStr(r.protocol), derefStr(r.portRange), derefStr(r.source), derefStr(r.desc)),
				Confidence: evidence.Fact,
			})
		}
	}

	for _, eni := range def.networkInterfaces {
		facts = append(facts, evidence.Evidence{
			Source: "ec2:DescribeInstances", ResourceID: resourceID, Field: "NetworkInterface",
			Value:      fmt.Sprintf("%s publicIp=%s privateIp=%s", derefStr(eni.id), derefStr(eni.publicIpv4Add), derefStr(eni.privateIpv4Add)),
			Confidence: evidence.Fact,
		})
	}

	return facts
}
