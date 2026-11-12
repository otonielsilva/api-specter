package cli

// commands lists the names suggest can propose. Keep it in sync with the
// switch in Run.
var commands = []string{"help", "plan", "run", "version"}

// suggest returns the command closest to name, or "" when none is close
// enough to be a likely typo. "Close enough" is an edit distance of at most
// 2, which catches swapped, missing or extra letters without proposing
// "run" for any three-letter word.
func suggest(name string) string {
	best, bestDist := "", 3
	for _, c := range commands {
		if d := levenshtein(name, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

// levenshtein returns the number of single-rune insertions, deletions and
// substitutions needed to turn a into b. It keeps one row of the dynamic
// programming table instead of the full matrix.
func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	row := make([]int, len(rb)+1)
	for j := range row {
		row[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		prev := row[0] // row[i-1][j-1]
		row[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur := min(row[j]+1, row[j-1]+1, prev+cost)
			prev, row[j] = row[j], cur
		}
	}
	return row[len(rb)]
}
