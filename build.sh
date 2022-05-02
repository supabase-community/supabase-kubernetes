#!/bin/bash
#



helm package charts/supabase -d build/
helm repo index build --url https://github.com/jorpilo/supabase-kubernetes/blob/main/build