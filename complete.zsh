#compdef _k k

function _k(){
    local line

    _arguments -C \
        '--cluster[Show clusters]' \
        '+[Show contexts]' \
        ':[Show namespaces]' \
        "*::arg:->args"

    case $line[1] in
        '@')
            __kubectl_config_get_clusters
        ;;
        '+')
            __kubectl_config_get_contexts
        ;;
        ':')
            __kubectl_config_get_namespaces
        ;;
    esac
}