package main

import (
	"fmt"
	"regexp"
)

type ReleaseLevel string

var (
	ReleaseLevelAlpha            ReleaseLevel = "a"
	ReleaseLevelBeta             ReleaseLevel = "b"
	ReleaseLevelReleaseCandidate ReleaseLevel = "rc"
	ReleaseLevelFinal            ReleaseLevel = ""

	pythonProjectVersionRegexp = regexp.MustCompile(
		`(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:(a|b|rc)(0|[1-9]\d*))?`,
	)
	pythonSerialToFileVersionMap = map[ReleaseLevel]uint16{
		ReleaseLevelAlpha:            0x2000,
		ReleaseLevelBeta:             0x4000,
		ReleaseLevelReleaseCandidate: 0x6000,
		ReleaseLevelFinal:            0x8000,
	}
)

type FileVersion struct {
	Major uint16
	Minor uint16
	Patch uint16
	Build uint16
}

func (fv FileVersion) String() string {
	return fmt.Sprintf("%v.%v.%v.%v", fv.Major, fv.Minor, fv.Patch, fv.Build)
}

type PythonProjectVersion struct {
	Major        uint16
	Minor        uint16
	Micro        uint16
	ReleaseLevel ReleaseLevel
	Serial       uint8
}

func parsePythonProjectVersion(v string) PythonProjectVersion {
	matches := pythonProjectVersionRegexp.FindStringSubmatch(v)
	if matches == nil {
		panic("could not parse Python project version")
	}
	ppv := PythonProjectVersion{}
	ppv.Major = ParseInt[uint16](matches[1])
	ppv.Minor = ParseInt[uint16](matches[2])
	ppv.Micro = ParseInt[uint16](matches[3])
	ppv.ReleaseLevel = ReleaseLevel(matches[4])
	if matches[5] != "" {
		ppv.Serial = ParseInt[uint8](matches[5])
	}
	return ppv
}

func (ppv PythonProjectVersion) String() string {
	baseVersion := fmt.Sprintf("%v.%v.%v", ppv.Major, ppv.Minor, ppv.Micro)
	if ppv.ReleaseLevel == ReleaseLevelFinal {
		return baseVersion
	}
	return fmt.Sprintf("%v%v%v", baseVersion, ppv.ReleaseLevel, ppv.Serial)
}

func (ppv PythonProjectVersion) FileVersion() FileVersion {
	return FileVersion{
		Major: ppv.Major,
		Minor: ppv.Minor,
		Patch: ppv.Micro,
		Build: pythonSerialToFileVersionMap[ppv.ReleaseLevel] | uint16(ppv.Serial),
	}
}
