#!/bin/bash
#

helm package charts/supabase -d build/
helm repo index build --url https://raw.githubusercontent.com/supabase-community/supabase-kubernetes/master/build