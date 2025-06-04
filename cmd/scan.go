package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

type FileNode struct {
	Name     string
	Path     string
	Size     int64
	IsDir    bool
	Children []*FileNode
}

var (
	scanPath   string
	maxDepth   int
	topN       int
	minSize    int64
	numThreads int
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan a directory and display its largest files and folders",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Scanning %s...\n", scanPath)
		root, err := buildTree(scanPath, 0, maxDepth, topN)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		fmt.Printf("📁 Tree for %s\n\n", scanPath)
		printTree(root, "", true)
	},
}

func init() {
	scanCmd.Flags().StringVarP(&scanPath, "path", "p", ".", "Path to scan")
	scanCmd.Flags().IntVarP(&maxDepth, "max-depth", "d", -1, "Max depth to scan (-1 = unlimited)")
	scanCmd.Flags().IntVarP(&topN, "top", "t", 0, "Top N largest items per folder (0 = all)")
	scanCmd.Flags().Int64Var(&minSize, "min-size", 0, "Minimum size (in bytes) to include in the tree")
	scanCmd.Flags().IntVar(&numThreads, "threads", 4, "Number of threads for scanning")
}

// Helpers

func getSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func buildTree(path string, depth, maxDepth, topN int) (*FileNode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	node := &FileNode{
		Name:  info.Name(),
		Path:  path,
		IsDir: info.IsDir(),
	}

	if !info.IsDir() {
		node.Size = info.Size()
		if node.Size < minSize {
			return nil, nil
		}
		return node, nil
	}

	if maxDepth >= 0 && depth >= maxDepth {
		size, _ := getSize(path)
		node.Size = size
		if node.Size < minSize {
			return nil, nil
		}
		return node, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return node, nil
	}

	var wg sync.WaitGroup
	results := make(chan *FileNode, len(entries))
	sem := make(chan struct{}, numThreads) // Semaphore to limit goroutines

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		wg.Add(1)
		sem <- struct{}{} // Acquire a slot
		go func(childPath string) {
			defer wg.Done()
			defer func() { <-sem }() // Release the slot
			child, err := buildTree(childPath, depth+1, maxDepth, topN)
			if err == nil && child != nil {
				results <- child
			}
		}(childPath)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for child := range results {
		node.Children = append(node.Children, child)
		node.Size += child.Size
	}

	if node.Size < minSize {
		return nil, nil
	}

	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Size > node.Children[j].Size
	})

	if topN > 0 && len(node.Children) > topN {
		node.Children = node.Children[:topN]
	}

	return node, nil
}

func printTree(node *FileNode, prefix string, isLast bool) {
	connector := "├──"
	nextPrefix := prefix + "│   "
	if isLast {
		connector = "└──"
		nextPrefix = prefix + "    "
	}

	folderColor := color.New(color.FgBlue, color.Bold).SprintFunc()
	fileColor := color.New(color.FgWhite).SprintFunc()

	sizeStr := humanize.Bytes(uint64(node.Size))
	sizeColor := color.New(color.FgYellow).SprintFunc()

	if node.Size > 1<<30 {
		sizeColor = color.New(color.FgRed).SprintFunc()
	}

	name := node.Name
	if node.IsDir {
		name = folderColor(name)
	} else {
		name = fileColor(name)
	}

	fmt.Printf("%s%s %s (%s)\n", prefix, connector, name, sizeColor(sizeStr))

	for i, child := range node.Children {
		printTree(child, nextPrefix, i == len(node.Children)-1)
	}
}
