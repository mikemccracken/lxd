package main

import (
	"fmt"
	"net/http"
	"syscall"

	"github.com/lxc/lxd/shared"
	"gopkg.in/lxc/go-lxc.v2"
)

var api10 = []Command{
	containersCmd,
	containerCmd,
	containerStateCmd,
	containerFileCmd,
	containerSnapshotsCmd,
	containerSnapshotCmd,
	containerExecCmd,
	aliasCmd,
	aliasesCmd,
	imageCmd,
	imagesCmd,
	imagesExportCmd,
	imagesSecretCmd,
	operationsCmd,
	operationCmd,
	operationWait,
	operationWebsocket,
	networksCmd,
	networkCmd,
	api10Cmd,
	certificatesCmd,
	certificateFingerprintCmd,
	profilesCmd,
	profileCmd,
}

func api10Get(d *Daemon, r *http.Request) Response {
	body := shared.Jmap{"api_compat": shared.APICompat}

	if d.isTrustedClient(r) {
		body["auth"] = "trusted"

		uname := syscall.Utsname{}
		if err := syscall.Uname(&uname); err != nil {
			return InternalError(err)
		}

		backing_fs, err := shared.GetFilesystem(d.lxcpath)
		if err != nil {
			return InternalError(err)
		}

		env := shared.Jmap{
			"lxc_version": lxc.Version(),
			"lxd_version": shared.Version,
			"driver":      "lxc",
			"backing_fs":  backing_fs}

		/*
		 * Based on: https://groups.google.com/forum/#!topic/golang-nuts/Jel8Bb-YwX8
		 * there is really no better way to do this, which is
		 * unfortunate. Also, we ditch the more accepted CharsToString
		 * version in that thread, since it doesn't seem as portable,
		 * viz. github issue #206.
		 */
		kernelVersion := ""
		for _, c := range uname.Release {
			if c == 0 {
				break
			}
			kernelVersion += string(byte(c))
		}

		env["kernel_version"] = kernelVersion
		body["environment"] = env

		serverConfig, err := getServerConfig(d)
		if err != nil {
			return InternalError(err)
		}

		config := shared.Jmap{}

		for key, value := range serverConfig {
			if key == "core.trust_password" {
				config[key] = true
			} else {
				config[key] = value
			}
		}

		body["config"] = config
	} else {
		body["auth"] = "untrusted"
	}

	return SyncResponse(true, body)
}

type apiPut struct {
	Config shared.Jmap `json:"config"`
}

func api10Put(d *Daemon, r *http.Request) Response {
	req := apiPut{}

	if err := shared.ReadToJSON(r.Body, &req); err != nil {
		return BadRequest(err)
	}

	for key, value := range req.Config {
		if !ValidServerConfigKey(key) {
			return BadRequest(fmt.Errorf("Bad server config key: '%s'", key))
		}

		if key == "core.trust_password" {
			err := setTrustPassword(d, value.(string))
			if err != nil {
				return InternalError(err)
			}
		} else if key == "core.lvm_vg_name" {
			err := setLVMVolumeGroupNameConfig(d, value.(string))
			if err != nil {
				return InternalError(err)
			}
		} else if key == "core.lvm_thinpool_name" {
			err := setLVMThinPoolNameConfig(d, value.(string))
			if err != nil {
				return InternalError(err)
			}
		} else {
			err := setServerConfig(d, key, value.(string))
			if err != nil {
				return InternalError(err)
			}
		}
	}

	return EmptySyncResponse
}

var api10Cmd = Command{name: "", untrustedGet: true, get: api10Get, put: api10Put}
