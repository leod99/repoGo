// Package render implements templates rendering, returns device acl map.
package render

import (
	"bytes"
	"path"
	"strings"
	"text/template"

	".../base/go/runfiles"

	spb ".../proto/stratus_proto"
)

// templateDir is the directory where acl templates are stored.
const templateDir = ".../lib/templates/"

// intfDir contains interface and direction info.
type intfDir struct {
	Intf      string
	Direction string
}

// acl contains the data for template execution.
type acl struct {
	Name          string
	IPVersion     int32
	Data          string
	IntfDirection []*intfDir
}

// loadTemplate returns a parsed template containing the given template file.
// This function will panic if the templates cannot be parsed.
func loadTemplate(templateName string) *template.Template {
	return template.Must(
		template.ParseFiles(
			path.Join(runfiles.Path(templateDir), templateName+".tmpl")))
}

// loadACL parses ACLPushItem pb to map.
func loadACL(pushPb []*spb.ACLPushItem) (canaryACL map[string][]*acl, globalACL map[string][]*acl, deviceVendor map[string]string) {
	var aclData, aclName, deviceName, vendor string
	var ipVer int32
	var canary bool
	// map device name to a list of Acl
	canaryMap := make(map[string][]*acl)
	globalMap := make(map[string][]*acl)

	// map device name to vendor
	vendorMap := make(map[string]string)

	//load pb to map
	for _, item := range pushPb {
		aclData = *item.Data
		aclName = *item.AclName
		vendor = *item.Vendor
		if vendor == "cisco" {
			aclName = strings.Split(aclName, ".")[2]
		}
		if item.IpVersion != nil {
			ipVer = *item.IpVersion
		}
		for _, enforcePoint := range item.EnforcePoint {
			deviceName = *enforcePoint.DeviceName
			vendorMap[deviceName] = vendor
			canary = false
			for _, tag := range enforcePoint.Tags {
				if (*tag.Type == "Canary") && (*tag.Value == "True") {
					canary = true
					break
				}
			}
			intfs := make([]*intfDir, 0, len(enforcePoint.Units))
			for _, unit := range enforcePoint.Units {
				//add intf, direction
				intfs = append(intfs, &intfDir{*unit.Name, *unit.Direction})
			}
			if canary {
				canaryMap[deviceName] = append(canaryMap[deviceName], &acl{aclName, ipVer, aclData, intfs})
				continue
			}
			globalMap[deviceName] = append(globalMap[deviceName], &acl{aclName, ipVer, aclData, intfs})
		}
	}
	return canaryMap, globalMap, vendorMap
}

// PreparePush returns canary and global acl map keyed on device name.
func PreparePush(pushPb []*spb.ACLPushItem) ([]map[string]string, []error) {
	var err error
	var errs []error
	canaryMap, globalMap, vendorMap := loadACL(pushPb)
	ciscoACLTmpl := loadTemplate("cisco_acl")
	aclMaps := []map[string][]*acl{canaryMap, globalMap}
	results := make([]map[string]string, 2, 2)
	for idx := range aclMaps {
		deviceACL := make(map[string]string)
		for dev := range aclMaps[idx] {
			deviceACL[dev], err = configHandler(vendorMap[dev], aclMaps[idx][dev], ciscoACLTmpl)
			if err != nil {
				errs = append(errs, err)
			}
		}
		results[idx] = deviceACL
	}
	return results, errs
}

// configHandler renders template, returns acl config as string.
func configHandler(vendor string, aclData []*acl, aclTmpl *template.Template) (string, error) {
	var config string
	b := bytes.NewBuffer([]byte{})
	if vendor == "cisco" {
		for _, data := range aclData {
			if err := aclTmpl.ExecuteTemplate(b, "layout", data); err != nil {
				return "", err
			}
		}
		config = b.String()
		config = strings.Replace(config, "\nexit", "", -1)
	} else if vendor == "juniper" {
		for _, data := range aclData {
			b.WriteString(data.Data)
		}
		config = b.String()
	}
	return config, nil
}
