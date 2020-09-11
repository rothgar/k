#/usr/bin/env bash
function __k_parse_config() {
    local template kubectl_out lead_char

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
        if kubectl_out=$(kubectl config view -o template --template="${template}"); then
            COMPREPLY=( $( compgen -W "${kubectl_out[*]}" -- "${cur##*,}" ) ); compopt -o nospace;
        fi
    # fi
}
function __k_parse_config_contexts() {
    __k_parse_config "contexts"
}
function __k_parse_config_clusters() {
    __k_parse_config "clusters"
}

function _k_completions() {

    local cur prev words cword;
    # set above variables
    _init_completion -s || return

    case $cur in
    +* )
        local prefix=
        [[ $cur == *,* ]] && prefix="${cur%,*},"
        __k_parse_config_contexts
        ((${#COMPREPLY[@]} == 1)) && COMPREPLY=(${COMPREPLY/#/$prefix})
        ;;
    @* )
        __k_parse_config_clusters
        ;;
    :* )
        __kubectl_get_resource_namespace
        ;;
    *:* )
        __kubectl_get_resource_namespace
        ;;
    * )
        ;;
    esac

}

complete -F _k_completions k