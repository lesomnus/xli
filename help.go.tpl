{{ with $.Tree -}}
	{{ $root := index . 0 -}}
	{{ $rest := slice . 1 -}}

Name:
{{ print "    " $root.Name -}}
	{{ range $rest -}}
		{{ print "." .Name -}}
	{{ end -}}
	{{ if $.Brief -}}
		{{ print " - " $.Brief -}}
	{{ end }}

Usage:
{{ print "    " (usage $) -}}
	{{ range . -}}
		{{ if len .Args | ne 0 -}}
			{{ printf "\n" -}}
		{{ end -}}

		{{ range .Args -}}
			{{ if len .Brief | ne 0 -}}
				{{ printf "\n  %s:\n    %s" .Name .Brief -}}
				{{ if .Info.HasDefault }}{{ printf " (default: %s)" .Info.Default -}}{{ end -}}
				{{ print "\n" -}}
			{{ end -}}
		{{ end -}}
	{{ end -}}
{{ end -}}
{{ if $.Synop }}

Description:
{{ printf "    %s" $.Synop -}}
{{ end -}}

{{ if len $.Commands.Visible | ne 0 }}

Commands:{{ range $.Commands.Visible.ByCategory -}}
		{{ $category := (index . 0).Category -}}
		{{ if len $category | ne 0 -}}
			{{ printf "\n  %s:" $category -}}
		{{ end -}}
		{{ range . -}}
			{{ printf "\n    %-20s %s" .String .Brief -}}
		{{ end -}}
	{{ end -}}
{{ end }}

Options:
{{- printf "\n    %-20s %s" "-h,--help" "show help" -}}
{{ range $.Flags.Visible.ByCategory -}}
		{{ $category := (index . 0).Info.Category -}}
		{{ if len $category | ne 0 -}}
			{{ printf "\n  %s:" $category -}}
		{{ end -}}
		{{ range . -}}{{ with .Info -}}
			{{ printf "\n    %-20s %s" .String .Brief -}}
			{{ if .Required }}{{ print " (required)" -}}{{ end -}}
			{{ if .HasDefault }}{{ printf " (default: %s)" .Default -}}{{ end -}}
		{{ end -}}{{ end -}}
	{{ end -}}

{{ print "\n" -}}
