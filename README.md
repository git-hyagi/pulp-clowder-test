podman build -t quay.io/rhn_support_hyagi/clowder-test .
podman push quay.io/rhn_support_hyagi/clowder-test


# just in case the registry is private
export serviceAccount=example-pulp
oc -npulp get secret quay-io || oc -npulp create secret docker-registry quay-io --from-file=.dockerconfigjson=${XDG_RUNTIME_DIR}/containers/auth.json
oc -npulp secret link $serviceAccount  quay-io  --for=pull

oc apply -f- <<EOF
apiVersion: v1
kind: Pod
metadata:
  name: clowder-test
spec:
  containers:
  - name: clowder
    image: quay.io/rhn_support_hyagi/clowder-test
  serviceAccount: $serviceAccount
  restartPolicy: Never
  imagePullPolicy: Always
EOF
