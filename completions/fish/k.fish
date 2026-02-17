# Source kubectl completion and rename all kubectl references to k
# This makes the standard kubectl completion work for the k command
kubectl completion fish | string replace -a kubectl k | source

# Custom completion for k's special syntax (@cluster, +context, :namespace)

# Complete contexts with + prefix
complete -c k -n '__fish_seen_argument -w +*' -a '(kubectl config view -o template --template="{{ range .contexts }}+{{ .name }}\n{{ end }}" 2>/dev/null)' -d 'context'

# Complete clusters with @ prefix
complete -c k -n '__fish_seen_argument -w @*' -a '(kubectl config view -o template --template="{{ range .clusters }}@{{ .name }}\n{{ end }}" 2>/dev/null)' -d 'cluster'

# Complete namespaces with : prefix
complete -c k -n '__fish_seen_argument -w :*' -a '(kubectl get namespaces -o template --template="{{ range .items }}:{{ .metadata.name }}\n{{ end }}" 2>/dev/null)' -d 'namespace'
