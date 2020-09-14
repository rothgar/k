#/usr/bin/env bash
function __k_parse_config() {
    local template k_out lead_char

    # There's probably a beter way to keep the + or @ after tab completion
    case $1 in
    contexts )
        lead_char='+' ;;
    clusters )
        lead_char='@' ;;
    esac

    # Use lead_char in output so each variable has it
    template="{{ range .$1  }}$lead_char{{ .name }} {{ end }}"

    # # Use this path if adding more clusters or contexts
    # if [[ "$cur" == *, ]]; then
    #     grep_exclude_pattern=$(echo $cur | tr -d $lead_char | tr ',' '|')
    #     if kubectl_out=$(__kubectl_debug_out "k config $(__kubectl_override_flags) -o template --template=\"${template}\" view" \
    #         grep -E -v $grep_exclude_pattern); then
    #         COMPREPLY=( $( compgen -W "${kubectl_out[*]}" ) ); compopt -o nospace;
    #     fi
    # else
        # ${cur##*,} removes everything up to and including ,
        if k_out=$(k config view -o template --template="${template}"); then
            COMPREPLY=( $( compgen -W "${k_out[*]}" -- "${cur##*,}" ) ); compopt -o nospace;
        fi
    # fi
}
__k_parse_get() {
    # echo $cur ${cur#:}
    local template
    template="${2:-"{{ range .items  }}{{ .metadata.name }} {{ end }}"}"
    local k_out
    if k_out=$(k get -o template --template="${template}" "$1"); then
        # remove the leading : for filtering compgen
        cur=${cur#*:}
        # ${cur##*,} removes everything up to the first comma so ":default,kube" becomes "kube"
        COMPREPLY=( $( compgen -W "${k_out[*]}" -- "${cur##*,}") ); compopt -o nospace;
    fi
}

__k_parse_config_contexts() {
    __k_parse_config "contexts"
}
__k_parse_config_clusters() {
    __k_parse_config "clusters"
}
__k_get_resource_namespace()
{
    __k_parse_get "namespace"
}

function _k_completions() {

    # set -x
    local cur prefix;
    # set the current arg
    # prefix is used if we need to match part of the arg but want to
    # return a complete line. +context:na<tab> should complete namespace
    # but we need the full line to have +context: as part of the prefix
    #
    # _init_completion doesn't like args that start with :
    # so we use _get_comp_words_by_ref and exclude : from COMP_WORDBREAKS
    # ":default" with _init_completion would become prev=: cur=default
    # in this case cur=:default
    _get_comp_words_by_ref -n : cur || return

    # try to find a pattern match for the current argument
    case $cur in
    # start with most specific matches and go toward general matches
    # because case is processed in order
    *,* )
        # remove a leading colon from the prefix
        # because otherwise each tab complete will add it to the front
        # because we're using _get_comp_words_by_ref -n :
        # ${cur#:} removes the shortest leading match which will only be
        # a leading : in this case
        cur=${cur#:}
        # $cur matches *,* so we want our prefix to be
        # everything before the last comma (adding a comma)
        prefix="${cur%,*},"
        # more $cur manipulation happens in this function
        __k_get_resource_namespace
        # add the prefix back to the matched value
        COMPREPLY=(${COMPREPLY/#/$prefix})
        ;;
    :* )
        __k_get_resource_namespace
        ;;
    *:* )
        [[ $cur == *,* ]] && prefix="${cur%,*},"
        __k_get_resource_namespace
        COMPREPLY=(${COMPREPLY/#/$prefix})
        ;;
    +* )
        [[ $cur == *,* ]] && prefix="${cur%,*},"
        __k_parse_config_contexts
        ((${#COMPREPLY[@]} != 1)) && COMPREPLY=(${COMPREPLY/#/$prefix})
        ;;
    @* )
        __k_parse_config_clusters
        ;;
    * )
        # echo \ngot $prev $cur and $words
        ;;
    esac
    # set +x

}

complete -F _k_completions k