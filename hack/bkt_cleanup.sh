# script to cleanup all artifacts of bucket creation.
# $1 = obc name (required), $2 = OBC's namespace (optional)
#
if (( $# == 0 || $# > 2 )); then
   echo
   echo "Cleans up the secret, configmap and OB for the given OBC"
   echo "Usage: $0 obc-name [obc-namespace]"
   exit 1
fi
errcnt=0
obcName="$1"; ns="$2"
if [[ -z "$obcName" ]]; then
   echo "OBC name is required"
   exit 1
fi
[[ -z "$ns" ]] && ns="default"

echo
echo "Cleaning up for OBC \"$ns/obcName\"..."
echo
kubectl get obc -n=$ns $obcName
(( $? != 0 )) && exit 1

echo
echo "delete secret $ns/$obcName..."
# secret and cm have finalizers which need to be removed or commented
if kubectl patch --type=merge secret -n=$ns $obcName -p '{"metadata":{"finalizers": [null]}}'; then
   kubectl delete secret -n=$ns $obcName
   (( $? != 0 )) && ((errcnt++))
else
   ((errcnt++))
fi

echo
echo "delete configmap $ns/$obcName..."
if kubectl patch --type=merge cm -n=$ns $obcName -p '{"metadata":{"finalizers": [null]}}'; then
   kubectl delete cm -n=$ns $obcName
   (( $? != 0 )) && ((errcnt++))
else
   ((errcnt++))
fi

echo
echo "delete ob obc-$ns-$obcName..."
kubectl delete ob "obc-$ns-$obcName"
(( $? != 0 )) && ((errcnt++))

echo
echo "end of cleanup with $errcnt errors"
