# grab the latest (master) bucket provisioning lib and vendor it.
sed -ri 's,(.*lib-bucket-provisioner).*,\1 master,' go.mod && vgo mod vendor
