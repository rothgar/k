# Source kubectl completion and rename all kubectl references to k
# This makes the standard kubectl completion work for the k command.
#
# kubectl's completion asks the binary itself (k __complete ...) for
# candidates. k turns kspace arguments into --context and --namespace before
# kubectl sees them, so resource completion after a kspace word is already
# context aware. Only the kspace words themselves need custom handling.
kubectl completion fish | string replace -a kubectl k | source

# Custom completion for k's special syntax (@cluster, +context, :namespace)

# Complete +context[,context] or @cluster[,cluster] from the kubeconfig k uses
function __k_kspace_config -a kind
    set -l tok (commandline -ct)
    # Everything up to the last comma, or just the leading +/@ character
    set -l prefix (string replace -r '[^,]*$' '' -- $tok)
    test -z "$prefix"; and set prefix (string sub -l 1 -- $tok)
    for name in (k config view -o template --template="{{ range .$kind }}{{ .name }}{{ \"\n\" }}{{ end }}" 2>/dev/null)
        echo "$prefix$name"
    end
end

# Complete :namespace, +context:namespace or @cluster:namespace (comma separated too)
function __k_kspace_namespaces
    set -l tok (commandline -ct)
    # target is the +context or @cluster part; no match (bare :namespace)
    # yields an empty list so no argument is passed to k
    set -l target (string match -r '^[+@][^:]+' -- $tok)
    # Everything up to the last : or ,
    set -l prefix (string replace -r '[^:,]*$' '' -- $tok)
    for name in (k $target get namespaces -o template --template='{{ range .items }}{{ .metadata.name }}{{ "\n" }}{{ end }}' 2>/dev/null)
        echo "$prefix$name"
    end
end

complete -c k -n 'string match -qr "^\+[^:]*\$" -- (commandline -ct)' -f -a '(__k_kspace_config contexts)' -d 'context'
complete -c k -n 'string match -qr "^@[^:]*\$" -- (commandline -ct)' -f -a '(__k_kspace_config clusters)' -d 'cluster'
complete -c k -n 'string match -qr "^([+@][^:]*)?:" -- (commandline -ct)' -f -a '(__k_kspace_namespaces)' -d 'namespace'
