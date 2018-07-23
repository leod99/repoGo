package render

import (
	"path"
	"reflect"
	"testing"

	".../file/base/go/file"
	".../go/context/context"
	".../net/proto2/go/proto"
	spb ".../proto/stratus_proto"
	".../testing/gobase/test"
)

var (
	aclPushPb = []*spb.ACLPushItem{
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String("ussvl2-1"),
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
			},
			AclName:   proto.String(".acl."),
			IpVersion: proto.Int32(4),
			Vendor:    proto.String("cisco"),
      Data:      proto.String("no ip access-list extended \nip access-list extended \n"),
		},
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String("jp-t1"),
					Units: []*spb.ACLEnforcementPoint_ACLEnforcementUnit{
						{
							Name:      proto.String("Vlan86"),
							Direction: proto.String("out"),
						},
					},
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("APAC"),
						},
						{
							Type:  proto.String("Canary"),
							Value: proto.String("True"),
						},
					},
				},
			},
			AclName:   proto.String("go_out"),
			IpVersion: proto.Int32(4),
			Vendor:    proto.String("cisco"),
      Data:      proto.String("no ip access-\n"),
		},
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String("jp1"),
					Units: []*spb.ACLEnforcementPoint_ACLEnforcementUnit{
						{
							Name:      proto.String("Vlan86"),
							Direction: proto.String("in"),
						},
					},
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("APAC"),
						},
						{
							Type:  proto.String("Canary"),
							Value: proto.String("True"),
						},
					},
				},
			},
			AclName:   proto.String("n"),
			IpVersion: proto.Int32(4),
			Vendor:    proto.String("cisco"),
      Data:      proto.String("no ip access\n"),
		},
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String("us1"),
					Units: []*spb.ACLEnforcementPoint_ACLEnforcementUnit{
						{
							Name:      proto.String("Vlan75"),
							Direction: proto.String("in"),
						},
						{
							Name:      proto.String("Vlan75"),
							Direction: proto.String("out"),
						},
					},
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
			},
			AclName:   proto.String("v3in"),
			IpVersion: proto.Int32(4),
			Vendor:    proto.String("cisco"),
      Data:      proto.String("no ip access-list extended goon\n"),
		},
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String("us-tv"),
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
				{
					DeviceName: proto.String("utv"),
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
				{
					DeviceName: proto.String("us-.mtv"),
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
			},
			AclName: proto.String("jcl"),
			Vendor:  proto.String("juniper"),
			Data:    proto.String("firewall {\n    family inet {\n        test juniper\n    }\n}\n"),
		},
		{
			EnforcePoint: []*spb.ACLEnforcementPoint{
				{
					DeviceName: proto.String(".mtv"),
					Tags: []*spb.ACLEnforcementPoint_Tag{
						{
							Type:  proto.String("Region"),
							Value: proto.String("AMER"),
						},
					},
				},
			},
			AclName: proto.String("u.srx"),
			Vendor:  proto.String("juniper"),
			Data:    proto.String("security {\n    replace: policies {\n        test srx\n    }\n}\n"),
		},
	}
)

func getConfigFromFile(fileName string, t *testing.T) string {
	path := path.Join(.TestSrcDir, ".../o/lib/testdata", fileName)
	ctx := context.Background()
	fileContent, err := file.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("File Open Error: %v", err)
	}
	return string(fileContent)
}

func TestPreparePush(t *testing.T) {
	// construct test data
	tests := []struct {
		name  string
		input []*spb.ACLPushItem
	}{
		{name: "no_ACLEnforcementUnit", input: aclPushPb[0:1]},
		{name: "multiple_ACL", input: aclPushPb[1:3]},
		{name: "multiple_ACLEnforcementUnit", input: aclPushPb[3:4]},
		{name: "jcl_srx_coexist", input: aclPushPb[4:6]},
	}

	for _, test := range tests {
		wantCanary := make(map[string]string)
		wantGlobal := make(map[string]string)
		acls, err := PreparePush(test.input)
		if err != nil {
			t.Errorf("PreparePush error: %v", err)
		}
		// read config from test file
		for dev := range acls[0] {
			wantCanary[dev] = getConfigFromFile(dev, t)
		}
		for dev := range acls[1] {
			wantGlobal[dev] = getConfigFromFile(dev, t)
		}
		if !reflect.DeepEqual(acls[0], wantCanary) {
      t.Errorf("Canary acl mismatch for test: %s, got: %v, want: %v", test.name, acls[0], wantCanary)
		}
		if !reflect.DeepEqual(acls[1], wantGlobal) {
      t.Errorf("Global acl mismatch for test: %s, got: %v, want: %v", test.name, acls[1], wantGlobal)
		}
	}

}
