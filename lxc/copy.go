package main

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/chai2010/gettext-go/gettext"

	"github.com/lxc/lxd"
	"github.com/lxc/lxd/shared"
)

type copyCmd struct {
	httpAddr string
}

func (c *copyCmd) showByDefault() bool {
	return true
}

func (c *copyCmd) usage() string {
	return gettext.Gettext(
		"Copy containers within or in between lxd instances.\n" +
			"\n" +
			"lxc copy [remote:]<source container> [remote:]<destination container>\n")
}

func (c *copyCmd) flags() {}

func copyContainer(config *lxd.Config, sourceResource string, destResource string, keepVolatile bool) error {
	sourceRemote, sourceName := config.ParseRemoteAndContainer(sourceResource)
	destRemote, destName := config.ParseRemoteAndContainer(destResource)

	if sourceName == "" {
		return fmt.Errorf(gettext.Gettext("you must specify a source container name"))
	}

	if destName == "" {
		destName = sourceName
	}

	source, err := lxd.NewClient(config, sourceRemote)
	if err != nil {
		return err
	}

	status := &shared.ContainerState{}

	// TODO: presumably we want to do this for copying snapshots too? We
	// need to think a bit more about how we track the baseImage in the
	// face of LVM and snapshots in general; this will probably make more
	// sense once that work is done.
	baseImage := ""

	if !shared.IsSnapshot(sourceName) {
		status, err = source.ContainerStatus(sourceName)
		if err != nil {
			return err
		}

		baseImage = status.Config["volatile.base_image"]

		if !keepVolatile {
			for k := range status.Config {
				if strings.HasPrefix(k, "volatile") {
					delete(status.Config, k)
				}
			}
		}
	}

	// Do a local copy if the remotes are the same, otherwise do a migration
	if sourceRemote == destRemote {
		if sourceName == destName {
			return fmt.Errorf(gettext.Gettext("can't copy to the same container name"))
		}

		cp, err := source.LocalCopy(sourceName, destName, status.Config, status.Profiles)
		if err != nil {
			return err
		}

		return source.WaitForSuccess(cp.Operation)
	} else {
		dest, err := lxd.NewClient(config, destRemote)
		if err != nil {
			return err
		}

		sourceProfs := shared.NewStringSet(status.Profiles)
		destProfs, err := dest.ListProfiles()
		if err != nil {
			return err
		}

		if !sourceProfs.IsSubset(shared.NewStringSet(destProfs)) {
			return fmt.Errorf(gettext.Gettext("not all the profiles from the source exist on the target"))
		}

		sourceWSResponse, err := source.GetMigrationSourceWS(sourceName)
		if err != nil {
			return err
		}

		secrets := map[string]string{}
		if err := json.Unmarshal(sourceWSResponse.Metadata, &secrets); err != nil {
			return err
		}

		addresses, err := source.Addresses()
		if err != nil {
			return err
		}

		for _, addr := range addresses {
			sourceWSUrl := "wss://" + addr + path.Join(sourceWSResponse.Operation, "websocket")

			var migration *lxd.Response
			migration, err = dest.MigrateFrom(destName, sourceWSUrl, secrets, status.Config, status.Profiles, baseImage)
			if err != nil {
				continue
			}

			if err = dest.WaitForSuccess(migration.Operation); err != nil {
				continue
			}

			return nil
		}

		return err
	}
}

func (c *copyCmd) run(config *lxd.Config, args []string) error {
	if len(args) != 2 {
		return errArgs
	}

	return copyContainer(config, args[0], args[1], false)
}
