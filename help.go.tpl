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
{{ print "    " -}}
	{{ range . -}}
		{{ print .Name " " -}}
		{{ range .Args -}}
			{{ print "<" .Name "> " -}}
		{{ end -}}
	{{ end -}}
	{{ if len $.Flags    | ne 0 }}[options] {{ end -}}
	{{ if len $.Commands | ne 0 }}[command]{{ end -}}
	{{ print "\n" -}}
	{{ range . -}}
		{{ range .Args -}}
			{{ if len .Brief | ne 0 -}}
				{{ printf "  %s:\n    %s\n" .Name .Brief -}}
			{{ end -}}
		{{ end -}}
	{{ end -}}
{{ end }}
{{ if len $.Commands | ne 0 -}}
Commands:{{ range $.Commands.ByCategory -}}
		{{ $category := (index . 0).Category -}}
		{{ if len $category | ne 0 -}}
			{{ printf "\n  %s:" $category -}}
		{{ end -}}
		{{ range . -}}
			{{ printf "\n    %-20s %s" .String .Brief -}}
		{{ end -}}
	{{ end -}}
{{ end }}

{{ if len $.Flags | ne 0 -}}
Options:{{ range $.Flags.ByCategory -}}
		{{ $category := (index . 0).Info.Category -}}
		{{ if len $category | ne 0 -}}
			{{ printf "\n  %s:" $category -}}
		{{ end -}}
		{{ range . -}}{{ with .Info -}}
			{{ printf "\n    %-20s %s" .String .Brief -}}
		{{ end -}}{{ end -}}
	{{ end -}}
{{ end }}

