package filter

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/prometheus/common/model"
)

var filters []Definition

// Definition for our parsed config file entries
type Definition struct {
	Label  string
	Filter string
	Value  string
}

// Init reads the config file adn (TODO!) registers a hook to reread it when it changes
func Init(file string) error {
	err := readConfig(file)
	if err != nil {
		return err
	}
	return nil
}

// Count returs the amount of filters
func Count() int {
	return len(filters)
}

func readConfig(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("Error reading config %s: %v", file, err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)

	var tempFilterList []Definition
	for scanner.Scan() {
		l := scanner.Text()

		if strings.HasPrefix(l, "#") {
			continue
		}
		tempFilterList = append(tempFilterList, parseLine(l))
	}

	filters = tempFilterList
	return nil
}

func parseLine(line string) Definition {
	elements := strings.SplitN(line, " ", 3)
	fd := Definition{
		Label:  elements[0],
		Filter: strings.ToUpper(elements[1]),
		Value:  elements[2],
	}
	return fd
}

// MatchesFilter checks if the metric matches the filters given in file
func MatchesFilter(metric model.Metric, logger log.Logger) (bool, error) {
	if len(filters) == 0 {
		level.Debug(logger).Log("msg", "MatchesFilter has no filters")
		return true, nil
	}

	if metric == nil {
		level.Debug(logger).Log("msg", "Empty metric, skipping filter")
		return false, nil
	}

	//level.Debug(logger).Log("msg", "Checking metric", "metric", metric.String())
	for _, filter := range filters {
		//level.Debug(logger).Log("msg", "Filter", "label", filter.Label, "value", filter.Value, "func", filter.Filter)
		switch filter.Filter {
		case "SI":
			r := startsWith(metric, filter.Label, filter.Value, true, logger)
			if r {
				return true, nil
			}
		case "SC":
			r := startsWith(metric, filter.Label, filter.Value, false, logger)
			if r {
				return true, nil
			}
		case "EI":
			r := equals(metric, filter.Label, filter.Value, true)
			if r {
				return true, nil
			}
		case "EC":
			r := equals(metric, filter.Label, filter.Value, false)
			if r {
				return true, nil
			}
		case "CI":
			r := contains(metric, filter.Label, filter.Value, true)
			if r {
				return true, nil
			}
		case "CC":
			r := contains(metric, filter.Label, filter.Value, false)
			if r {
				return true, nil
			}
		}
	}

	return false, nil
}

func contains(metric model.Metric, label string, value string, insensitive bool) bool {
	if insensitive {
		value = strings.ToLower(value)
		label = strings.ToLower(label)
	}

	for n, m := range metric {
		if string(n) != label {
			continue
		}

		q := string(m)
		if insensitive {
			q = strings.ToLower(q)
		}

		if strings.Contains(q, value) {
			return true
		}
	}

	return false
}

func equals(metric model.Metric, label string, value string, insensitive bool) bool {
	if insensitive {
		value = strings.ToLower(value)
		label = strings.ToLower(label)
	}

	for n, m := range metric {
		if string(n) != label {
			continue
		}

		q := string(m)
		if insensitive {
			q = strings.ToLower(q)
		}

		if q == value {
			return true
		}
	}

	return false
}

func startsWith(metric model.Metric, label string, value string, insensitive bool, logger log.Logger) bool {
	if insensitive {
		value = strings.ToLower(value)
		label = strings.ToLower(label)
	}

	for n, m := range metric {
		//level.Debug(logger).Log("value", value, "label", label, "labelname", n, "labelvalue", m)

		if string(n) != label {
			continue
		}

		q := string(m)
		if insensitive {
			q = strings.ToLower(q)
		}

		if strings.HasPrefix(q, value) {
			return true
		}
	}

	return false
}
