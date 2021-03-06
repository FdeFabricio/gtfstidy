// Copyright 2016 Patrick Brosi
// Authors: info@patrickbrosi.de
//
// Use of this source code is governed by a GPL v2
// license that can be found in the LICENSE file

package main

import (
	"fmt"
	"github.com/patrickbr/gtfsparser"
	"github.com/patrickbr/gtfstidy/processors"
	"github.com/patrickbr/gtfswriter"
	flag "github.com/spf13/pflag"
	"os"
	"path"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "gtfstidy - 2016 by P. Brosi\n\nUsage:\n\n  %s [-o <outputfile>] <input GTFS>\n\nAllowed options:\n\n", os.Args[0])
		flag.PrintDefaults()
	}

	onlyValidate := flag.BoolP("validation-mode", "v", false, "only validate the feed, no processors will be called")

	outputPath := flag.StringP("output", "o", "gtfs-out", "gtfs output directory or zip file (must end with .zip)")

	useDefaultValuesOnError := flag.BoolP("default-on-errs", "e", false, "if non-required fields have errors, fall back to the default values")
	dropErroneousEntities := flag.BoolP("drop-errs", "D", false, "drop erroneous entries from feed")

	useOrphanDeleter := flag.BoolP("delete-orphans", "O", false, "remove entities that are not referenced anywhere")
	useShapeMinimizer := flag.BoolP("min-shapes", "s", false, "minimize shapes (using Douglas-Peucker)")
	useShapeRemeasurer := flag.BoolP("remeasure-shapes", "m", false, "remeasure shapes (filling measurement-holes)")
	useRedShapeRemover := flag.BoolP("remove-red-shapes", "S", false, "remove shape duplicates")
	useRedRouteMinimizer := flag.BoolP("remove-red-routes", "R", false, "remove route duplicates")
	useRedServiceMinimizer := flag.BoolP("remove-red-services", "C", false, "remove duplicate services in calendar.txt and calendar_dates.txt")
	useIDMinimizerNum := flag.BoolP("minimize-ids-num", "i", false, "minimize IDs using numerical IDs (e.g. 144, 145, 146...)")
	useIDMinimizerChar := flag.BoolP("minimize-ids-char", "d", false, "minimize IDs using character IDs (e.g. abc, abd, abe, abf...)")
	useServiceMinimizer := flag.BoolP("minimize-services", "c", false, "minimize services by searching for the optimal exception/range coverage")
	useFrequencyMinimizer := flag.BoolP("minimize-stoptimes", "T", false, "search for frequency patterns in explicit trips and combine them, using a CAP approach")
	help := flag.BoolP("help", "?", false, "this message")

	flag.Parse()

	if *help {
		flag.Usage()
		return
	}

	gtfsPath := flag.Arg(0)

	if len(gtfsPath) == 0 {
		fmt.Fprintln(os.Stderr, "No GTFS location specified, see --help")
		os.Exit(1)
	}

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "Error:", r)
		}
	}()

	feed := gtfsparser.NewFeed()
	opts := gtfsparser.ParseOptions{UseDefValueOnError: false, DropErroneous: false, DryRun: *onlyValidate}
	opts.DropErroneous = *dropErroneousEntities && !*onlyValidate
	opts.UseDefValueOnError = *useDefaultValuesOnError && !*onlyValidate
	feed.SetParseOpts(opts)

	fmt.Fprintf(os.Stdout, "Parsing GTFS feed in '%s' ...", gtfsPath)
	e := feed.Parse(gtfsPath)

	if e != nil {
		fmt.Fprintf(os.Stderr, "\nError while parsing GTFS feed:\n")
		fmt.Fprintln(os.Stderr, e.Error())
		if !*onlyValidate {
			fmt.Fprintln(os.Stdout, "\nYou may want to try running gtfstidy with -e and/or -D for error fixing / skipping. See --help for details.")
		}
		os.Exit(1)
	} else {
		fmt.Fprintf(os.Stdout, " done.\n")
		minzers := make([]processors.Processor, 0)

		if *useOrphanDeleter {
			minzers = append(minzers, processors.OrphanRemover{})
		}

		if *useShapeRemeasurer || *useShapeMinimizer || *useRedShapeRemover {
			minzers = append(minzers, processors.ShapeRemeasurer{})
		}

		if *useShapeMinimizer {
			minzers = append(minzers, processors.ShapeMinimizer{Epsilon: 1.0})
		}

		if *useRedShapeRemover {
			minzers = append(minzers, processors.ShapeDuplicateRemover{MaxEqDistance: 10.0})
		}

		if *useRedRouteMinimizer {
			minzers = append(minzers, processors.RouteDuplicateRemover{})
		}

		if *useRedServiceMinimizer {
			minzers = append(minzers, processors.ServiceDuplicateRemover{})
		}

		if *useServiceMinimizer {
			minzers = append(minzers, processors.ServiceMinimizer{})
		}

		if *useFrequencyMinimizer {
			minzers = append(minzers, processors.FrequencyMinimizer{})
		}

		if *useIDMinimizerNum {
			minzers = append(minzers, processors.IDMinimizer{Base: 10})
		} else if *useIDMinimizerChar {
			minzers = append(minzers, processors.IDMinimizer{Base: 36})
		}

		if *onlyValidate {
			fmt.Fprintln(os.Stdout, "No errors.")
			os.Exit(0)
		} else {
			// do processing
			for _, m := range minzers {
				m.Run(feed)
			}

			fmt.Fprintf(os.Stdout, "Outputting GTFS feed to '%s'...", *outputPath)

			if _, err := os.Stat(*outputPath); os.IsNotExist(err) {
				if path.Ext(*outputPath) == ".zip" {
					os.Create(*outputPath)
				} else {
					os.Mkdir(*outputPath, os.ModePerm)
				}
			}

			// write feed back to output
			w := gtfswriter.Writer{ZipCompressionLevel: 9, Sorted: true}
			e := w.Write(feed, *outputPath)

			if e != nil {
				fmt.Fprintf(os.Stderr, "\nError while writing GTFS feed in '%s':\n ", *outputPath)
				fmt.Fprintln(os.Stderr, e.Error())
				os.Exit(1)
			}

			fmt.Fprintf(os.Stdout, " done.\n")
		}
	}
}
