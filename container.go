package main

import (
	"sort"
	"strconv"
	"strings"
	"time"

	. "github.com/byrnedo/dockdash/logger"
	goDocker "github.com/fsouza/go-dockerclient"
)

type container struct {
	*goDocker.Container
}

type containerSlice []container

func (cs containerSlice) sort() {
	sort.Slice(cs, func(i int, j int) bool {
		return cs[i].State.StartedAt.After(cs[j].State.StartedAt)
	})
}

type containerMap map[string]container

func (cm containerMap) toSlice() containerSlice {
	s := containerSlice(toSlice(cm))
	s.sort()
	return s
}

func toSlice[U comparable, V any](m map[U]V) (sorted []V) {
	sorted = make([]V, len(m))
	var i = 0
	for _, val := range m {
		sorted[i] = val
		i++
	}
	return
}

func (cm containerMap) getNameAndInfoOfContainers(offset int, infoType dockerInfoType, inspectMode bool) ([]string, []string) {
	var numContainers = len(cm)
	if offset > numContainers {
		offset = numContainers - 1
	}

	var (
		info                []string
		numContainersSubset = numContainers - offset
		names               = make([]string, numContainersSubset)
		containersSorted    = cm.toSlice()
		nameStr             = ""
		containerNumber     = 0
	)

	if !inspectMode {
		info = make([]string, numContainersSubset)
	}

	for index, cont := range containersSorted {
		if index < offset {
			continue
		}

		containerNumber = numContainers - index
		nameStr = strconv.Itoa(containerNumber) + ". " + cont.ID[:12] + " " + strings.TrimLeft(cont.Name, "/")

		if inspectMode && index == offset {
			names[index-offset] = "*" + nameStr
			info = cont.createInspectModeData(index, offset, infoType)
		} else {
			names[index-offset] = " " + nameStr
			if !inspectMode {
				info[index-offset] = cont.createRegularModeData(index, offset, infoType)
			}
		}

	}
	return names, info
}

func (cont container) createRegularModeData(index int, offset int, infoType dockerInfoType) (info string) {

	switch infoType {
	case ImageInfo:
		info = cont.Config.Image
	case Names:
		info = cont.Name
		if cont.Node != nil {
			info = cont.Node.Name + info
		}
	case PortInfo:
		info = strings.Join(createPortsSlice(cont.NetworkSettings.Ports), ",")
	case BindInfo:
		info = strings.TrimRight(strings.Join(cont.HostConfig.Binds, ","), ",")
	case CommandInfo:
		info = cont.Path + " " + strings.Join(cont.Args, " ")
	case EnvInfo:
		info = strings.TrimRight(strings.Join(cont.Config.Env, ","), ",")
	case EntrypointInfo:
		info = strings.Join(cont.Config.Entrypoint, " ")
	case VolumesInfo:
		volStr := ""
		for intVol, hostVol := range cont.Volumes {
			volStr += intVol + ":" + hostVol + ","
		}
		info = strings.TrimRight(volStr, ",")
	case TimeInfo:
		info = cont.State.StartedAt.Format(time.RubyDate)
	default:
		Error.Println("Unhandled info type", infoType)
	}
	return
}

func (cont container) createInspectModeData(index int, offset int, infoType dockerInfoType) (info []string) {
	switch infoType {
	case ImageInfo:
		info = []string{cont.Config.Image}
	case Names:
		if cont.Node != nil {
			info = []string{cont.Node.Name, cont.Name}
		} else {
			info = []string{cont.Name}
		}
	case PortInfo:
		info = createPortsSlice(cont.NetworkSettings.Ports)
	case BindInfo:
		info = make([]string, len(cont.HostConfig.Binds))
		for i, binding := range cont.HostConfig.Binds {
			info[i] = binding
		}
	case CommandInfo:
		info = make([]string, len(cont.Args))
		for i, arg := range cont.Args {
			info[i] = arg
		}
	case EnvInfo:
		info = make([]string, len(cont.Config.Env))
		for i, env := range cont.Config.Env {
			info[i] = env
		}
	case EntrypointInfo:
		info = make([]string, len(cont.Config.Entrypoint))
		for i, entrypoint := range cont.Config.Entrypoint {
			info[i] = entrypoint
		}
	case VolumesInfo:
		info = make([]string, len(cont.Volumes))
		i := 0
		for intVol, hostVol := range cont.Volumes {
			info[i] = intVol + ":" + hostVol + ""
			i++
		}
	case TimeInfo:
		info = []string{cont.State.StartedAt.Format(time.RubyDate)}
	default:
		Error.Println("Unhandled info type", infoType)
	}
	return
}

func createPortsSlice(ports map[goDocker.Port][]goDocker.PortBinding) (portsSlice []string) {

	portsSlice = make([]string, len(ports))
	i := 0
	for intPort, extHostPortList := range ports {
		if len(extHostPortList) == 0 {
			portsSlice[i] = intPort.Port() + "->N/A"
		}
		for _, extHostPort := range extHostPortList {
			portsSlice[i] = intPort.Port() + "->" + extHostPort.HostIP + ":" + extHostPort.HostPort
		}
		i++
	}
	return
}
