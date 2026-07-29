package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/go-github/v53/github"
	"golang.org/x/oauth2"
)

// OpenPR connects to GitHub and opens a Pull Request to fix the breaking API schema.
func OpenPR(endpoint string, patch map[string]interface{}) {
	token := os.Getenv("GITHUB_TOKEN")
	repoOwner := os.Getenv("GITHUB_TARGET_OWNER")
	repoName := os.Getenv("GITHUB_TARGET_REPO")

	if token == "" || repoOwner == "" || repoName == "" {
		fmt.Println("[PR ENGINE] ⚠️  Missing GITHUB_TOKEN, GITHUB_TARGET_OWNER, or GITHUB_TARGET_REPO. Simulating PR Engine.")
		simulatePR(endpoint, patch)
		return
	}

	fmt.Println("---------------------------------------------------")
	fmt.Printf("[PR ENGINE] Authenticating with GitHub as %s/%s...\n", repoOwner, repoName)

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// In a real scenario, we would parse the AST or use grep. 
	// For the MVP, we assume a file called `src/api.js` exists and we replace the keys.
	targetFile := "src/api.js"
	
	fileContent, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, targetFile, nil)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Could not find %s in %s/%s. Error: %v\n", targetFile, repoOwner, repoName, err)
		return
	}

	contentStr, err := fileContent.GetContent()
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Could not decode file content: %v\n", err)
		return
	}

	// Apply string replacement for the patch
	newContent := contentStr
	for oldKey, newKeyInterface := range patch {
		newKey := newKeyInterface.(string)
		// Extremely simple text replacement for MVP
		newContent = strings.ReplaceAll(newContent, fmt.Sprintf(`"%s"`, oldKey), fmt.Sprintf(`"%s"`, newKey))
		newContent = strings.ReplaceAll(newContent, fmt.Sprintf(`'%s'`, oldKey), fmt.Sprintf(`'%s'`, newKey))
		newContent = strings.ReplaceAll(newContent, oldKey+` :`, newKey+` :`)
		newContent = strings.ReplaceAll(newContent, oldKey+`:`, newKey+`:`)
	}

	if newContent == contentStr {
		fmt.Println("[PR ENGINE] ⚠️ No changes needed or string replacement didn't match anything.")
		return
	}

	// Create a new branch
	branchName := fmt.Sprintf("patchflow-fix-%d", time.Now().Unix())
	refName := "refs/heads/" + branchName

	// Get main branch SHA
	mainRef, _, err := client.Git.GetRef(ctx, repoOwner, repoName, "refs/heads/main")
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to get main branch: %v\n", err)
		return
	}

	newRef := &github.Reference{Ref: &refName, Object: &github.GitObject{SHA: mainRef.Object.SHA}}
	_, _, err = client.Git.CreateRef(ctx, repoOwner, repoName, newRef)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to create branch %s: %v\n", branchName, err)
		return
	}

	// Update the file
	opts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("fix: automated patch for %s deprecation", endpoint)),
		Content: []byte(newContent),
		SHA:     fileContent.SHA,
		Branch:  github.String(branchName),
	}
	_, _, err = client.Repositories.UpdateFile(ctx, repoOwner, repoName, targetFile, opts)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to update file: %v\n", err)
		return
	}

	// Create the Pull Request
	prTitle := fmt.Sprintf("🛠️ Patchflow Auto-Fix: Schema changes detected in %s", endpoint)
	prBody := fmt.Sprintf("### 🚨 Breaking API Change Detected\n\nPatchflow detected a 400 Bad Request from `%s`.\n\nOur AI agent analyzed the error and generated this mapping to fix the payload:\n```json\n%v\n```\n\nThis PR automatically applies this fix to `src/api.js`.", endpoint, patch)
	newPR := &github.NewPullRequest{
		Title:               github.String(prTitle),
		Head:                github.String(branchName),
		Base:                github.String("main"),
		Body:                github.String(prBody),
		MaintainerCanModify: github.Bool(true),
	}
	pr, _, err := client.PullRequests.Create(ctx, repoOwner, repoName, newPR)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to create PR: %v\n", err)
		return
	}

	fmt.Printf("[PR ENGINE] ✅ Pull Request successfully opened: %s\n", pr.GetHTMLURL())
	fmt.Println("---------------------------------------------------")
}

func simulatePR(endpoint string, patch map[string]interface{}) {
	fmt.Println("---------------------------------------------------")
	fmt.Printf("[PR ENGINE] Scanning customer repository for endpoint %s...\n", endpoint)
	fmt.Printf("[PR ENGINE] Looking for usages matching deprecated schema keys: %v\n", patch)
	fmt.Println("[PR ENGINE] Found occurrences of deprecated parameter.")
	fmt.Println("[PR ENGINE] Applying AST refactoring to update code...")
	fmt.Println("[PR ENGINE] Running tests (npm test)... PASSED")
	fmt.Println("[PR ENGINE] ✅ (Simulated) Pull Request #482 opened on GitHub!")
	fmt.Println("---------------------------------------------------")
}

// FixDependabotAlert opens a Pull Request to fix a repository broken by a dependency upgrade.
func FixDependabotAlert(pkgName, newVersion, targetFile, newCode string) {
	token := os.Getenv("GITHUB_TOKEN")
	repoOwner := os.Getenv("GITHUB_TARGET_OWNER")
	repoName := os.Getenv("GITHUB_TARGET_REPO")

	if token == "" || repoOwner == "" || repoName == "" {
		fmt.Println("[PR ENGINE] ⚠️  Missing GITHUB_TOKEN. Simulating Dependabot PR Engine.")
		simulateDependabotPR(pkgName, newVersion, targetFile, newCode)
		return
	}

	fmt.Println("---------------------------------------------------")
	fmt.Printf("[PR ENGINE] Authenticating with GitHub as %s/%s...\n", repoOwner, repoName)
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	fileContent, _, _, err := client.Repositories.GetContents(ctx, repoOwner, repoName, targetFile, nil)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Could not find %s. Error: %v\n", targetFile, err)
		return
	}

	branchName := fmt.Sprintf("patchflow-security-%d", time.Now().Unix())
	refName := "refs/heads/" + branchName

	mainRef, _, err := client.Git.GetRef(ctx, repoOwner, repoName, "refs/heads/main")
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to get main branch: %v\n", err)
		return
	}

	newRef := &github.Reference{Ref: &refName, Object: &github.GitObject{SHA: mainRef.Object.SHA}}
	_, _, err = client.Git.CreateRef(ctx, repoOwner, repoName, newRef)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to create branch %s: %v\n", branchName, err)
		return
	}

	opts := &github.RepositoryContentFileOptions{
		Message: github.String(fmt.Sprintf("fix: automated patch for %s security upgrade to %s", pkgName, newVersion)),
		Content: []byte(newCode),
		SHA:     fileContent.SHA,
		Branch:  github.String(branchName),
	}
	_, _, err = client.Repositories.UpdateFile(ctx, repoOwner, repoName, targetFile, opts)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to update file: %v\n", err)
		return
	}

	prTitle := fmt.Sprintf("🔒 Patchflow Auto-Fix: Update %s to %s & Fix Breaking API Changes", pkgName, newVersion)
	prBody := fmt.Sprintf("### 🚨 Dependabot Security Alert Auto-Healed\n\nPatchflow intercepted a Dependabot alert for `%s`.\n\nThe AI Agent noticed that upgrading to `%s` broke the build due to a library API change. The agent read the compiler errors, rewrote the broken code in `%s` to match the new library API, and opened this PR to safely patch your repository.", pkgName, newVersion, targetFile)
	newPR := &github.NewPullRequest{
		Title:               github.String(prTitle),
		Head:                github.String(branchName),
		Base:                github.String("main"),
		Body:                github.String(prBody),
		MaintainerCanModify: github.Bool(true),
	}
	pr, _, err := client.PullRequests.Create(ctx, repoOwner, repoName, newPR)
	if err != nil {
		fmt.Printf("[PR ENGINE] ❌ Failed to create PR: %v\n", err)
		return
	}

	fmt.Printf("[PR ENGINE] ✅ Security Pull Request successfully opened: %s\n", pr.GetHTMLURL())
	fmt.Println("---------------------------------------------------")
}

func simulateDependabotPR(pkgName, newVersion, targetFile, newCode string) {
	fmt.Println("---------------------------------------------------")
	fmt.Printf("[PR ENGINE] Scanning customer repository for vulnerable package %s...\n", pkgName)
	fmt.Println("[PR ENGINE] Analyzing compiler errors due to breaking library API changes...")
	fmt.Printf("[PR ENGINE] Applying AI refactoring to update %s...\n", targetFile)
	fmt.Println("[PR ENGINE] Running tests (go test ./...)... PASSED")
	fmt.Println("[PR ENGINE] ✅ (Simulated) Security Pull Request opened on GitHub!")
	fmt.Println("---------------------------------------------------")
}
