package git

import (
	"io/ioutil"
	"os"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/transport/ssh"
)

var gitCloneMasterFunc = gitCloneMaster

const repositoryDirPrefix = "oec"

func CloneMaster(url, privateKeyFilepath, passPhrase string) (repositoryPath string, err error) {

	tmpDir, err := ioutil.TempDir("", repositoryDirPrefix)
	if err != nil {
		return "", err
	}

	err = gitCloneMasterFunc(tmpDir, url, privateKeyFilepath, passPhrase)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

func gitCloneMaster(tmpDir, gitUrl, privateKeyFilepath, passPhrase string) error {

	options := &git.CloneOptions{
		URL:               gitUrl,
		RecurseSubmodules: git.DefaultSubmoduleRecursionDepth, // todo restrict max depth
		ReferenceName:     plumbing.Master,
		SingleBranch:      true,
	}

	if privateKeyFilepath != "" {

		auth, err := ssh.NewPublicKeysFromFile(ssh.DefaultUsername, privateKeyFilepath, passPhrase)
		if err != nil {
			return err
		}

		options.Auth = auth
	}

	_, err := git.PlainClone(tmpDir, false, options)

	return err
}
