// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package main

import (
	"context"
	"fmt"

	"github.com/Azure/acr-cli/cmd/api"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/memory"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/retry"
)

const (
	copPatchCmdLongMessage = `acr copa: patches all images in registry.`
)

// Besides the registry name and authentication information only the repository is needed.
type copaParameters struct {
	*rootParameters
	filters       []string
	filterTimeout uint64
}

// The tag command can be used to either list tags or delete tags inside a repository.
// that can be done with the tag list and tag delete commands respectively.
func newCopaPatchCmd(rootParams *rootParameters) *cobra.Command {
	copaParams := copaParameters{rootParameters: rootParams}
	cmd := &cobra.Command{
		Use:   "cssc",
		Short: "Patches repo inside a registry",
		Long:  copPatchCmdLongMessage,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			registryName, err := copaParams.GetRegistryName()
			loginURL := api.LoginURL(registryName)
			println(loginURL)
			println(registryName)
			//fs, err := file.New("/tmp/")
			// An acrClient with authentication is generated, if the authentication cannot be resolved an error is returned.
			acrClient, err := api.GetAcrCLIClientWithAuth(loginURL, copaParams.username, copaParams.password, copaParams.configs)
			if err != nil {
				return err
			}

			repo, err := remote.NewRepository(registryName + "/ocirepocollection:latest")
			if err != nil {
				panic(err)
			}

			repo.Client = &auth.Client{
				Client: retry.DefaultClient,
				Cache:  auth.DefaultCache,
				Credential: auth.StaticCredential(registryName, auth.Credential{
					Username: copaParams.username,
					Password: copaParams.password,
				}),
			}

			println("authentication done")
			println(repo.Reference.Registry)
			dst := memory.New()
			desc, err := oras.ExtendedCopy(ctx, repo, "latest", dst, "latest", oras.DefaultExtendedCopyOptions)
			println("after calling extended copy")
			if err != nil {
				panic(err)
			}

			println(desc.Digest.String())

			// op, error := dst.(ctx, desc)
			// tmpFile, err := ioutil.TempFile("", "example")
			// if err != nil {
			// 	fmt.Println("Error creating temporary file:", err)
			// }
			// defer tmpFile.Close()

			// // Write the memory stream data to the temporary file
			// _, err = tmpFile.Write(op)
			// if err != nil {
			// 	fmt.Println("Error writing to temporary file:", err)
			// }

			tagFilters, err := collectTagFilters(ctx, copaParams.filters, acrClient.AutorestClient, copaParams.filterTimeout)
			if err != nil {
				return err
			}

			fmt.Println("Printing all values in tagFilters:")
			for repoName, tagRegex := range tagFilters {
				fmt.Printf("Repository: %s, Tag Regex: %s\n", repoName, tagRegex)
			}
			if err != nil {
				return err
			}

			// cmd.AddCommand(
			// 	copaCmd,
			// )
			return nil
		},
	}

	cmd.Flags().StringArrayVarP(&copaParams.filters, "filter", "f", nil, "Specify the repository and a regular expression filter for the tag name, if a tag matches the filter and is older than the duration specified in ago it will be deleted. Note: If backtracking is used in the regexp it's possible for the expression to run into an infinite loop. The default timeout is set to 1 minute for evaluation of any filter expression. Use the '--filter-timeout-seconds' option to set a different value.")
	cmd.Flags().Uint64Var(&copaParams.filterTimeout, "filter-timeout-seconds", defaultRegexpMatchTimeoutSeconds, "This limits the evaluation of the regex filter, and will return a timeout error if this duration is exceeded during a single evaluation. If written incorrectly a regexp filter with backtracking can result in an infinite loop.")
	cmd.MarkFlagRequired("filter")
	return cmd

}

// newTagListCmd creates tag list command, it does not need any aditional parameters.
// The registry interaction is done through the listTags method
func copaPatchCmd(copaParams *copaParameters) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patch",
		Short: "patch all images in a registry",
		Long:  newTagListCmdLongMessage,
		RunE: func(cmd *cobra.Command, args []string) error {
			registryName, err := copaParams.GetRegistryName()
			if err != nil {
				return err
			}
			loginURL := api.LoginURL(registryName)
			// An acrClient is created to make the http requests to the registry.
			acrClient, err := api.GetAcrCLIClientWithAuth(loginURL, copaParams.username, copaParams.password, copaParams.configs)
			if err != nil {
				return err
			}
			ctx := context.Background()

			err = listRepositories(ctx, acrClient, loginURL)
			if err != nil {
				return err
			}
			return nil
		},
	}
	return cmd
}

// listTagss will do the http requests and print the digest of all the tags in the selected repository.
func listRepositories(ctx context.Context, acrClient api.AcrCLIClientInterface, loginURL string) error {
	resultRepositories, err := acrClient.GetAcrRepositories(ctx, "", nil)
	if err != nil {
		return errors.Wrap(err, "failed to list repositories")
	}

	if resultRepositories != nil {
		repositories := *resultRepositories.Names
		for _, repository := range repositories {
			fmt.Printf("%s\n", repository)
			listTags(ctx, acrClient, loginURL, repository)
		}
	}
	return nil

}
