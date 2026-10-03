# Based on supabase/docker/volumes/pooler/pooler.exs. Keep credentials out of
# error output: Ecto changesets and exception messages can contain input data.
try do
  {:ok, _} = Application.ensure_all_started(:supavisor)
  %{rows: [[version]]} = Supavisor.Repo.query!("select version()")
  {:ok, version} = Supavisor.Helpers.parse_pg_version(version)

  integer = fn name -> System.fetch_env!(name) |> String.to_integer() end
  user = %{
    "db_user" => "pgbouncer",
    "db_password" => System.fetch_env!("POSTGRES_PASSWORD"),
    "mode_type" => System.fetch_env!("POOLER_POOL_MODE"),
    "pool_size" => integer.("POOLER_DEFAULT_POOL_SIZE"),
    "is_manager" => true
  }
  params = %{
    "external_id" => System.fetch_env!("POOLER_TENANT_ID"),
    "db_host" => System.fetch_env!("POSTGRES_HOST"),
    "db_port" => integer.("POSTGRES_PORT"),
    "db_database" => System.fetch_env!("POSTGRES_DB"),
    "require_user" => false,
    "auth_query" => "SELECT * FROM pgbouncer.get_auth($1)",
    "default_max_clients" => integer.("POOLER_MAX_CLIENT_CONN"),
    "default_pool_size" => integer.("POOLER_DEFAULT_POOL_SIZE"),
    "default_parameter_status" => %{"server_version" => version},
    "users" => [user]
  }

  result =
    case Supavisor.Tenants.get_tenant_by_external_id(params["external_id"]) do
      nil ->
        Supavisor.Tenants.create_tenant(params)

      tenant ->
        # Supply association IDs so Ecto updates users in place. Preserve other
        # users rather than deleting them through the association's on_replace.
        users = Enum.map(tenant.users, fn existing ->
          attrs = existing
            |> Map.take([:id, :db_user, :db_user_alias, :db_password, :mode_type,
                         :pool_size, :is_manager, :pool_checkout_timeout, :max_clients])
            |> Map.new(fn {key, value} -> {Atom.to_string(key), value} end)

          if existing.db_user == "pgbouncer", do: Map.merge(attrs, user), else: attrs
        end)
        users = if Enum.any?(tenant.users, &(&1.db_user == "pgbouncer")),
          do: users, else: users ++ [user]
        params = Map.put(params, "users", users)
        changeset = Supavisor.Tenants.Tenant.changeset(tenant, params)

        if changeset.valid? && changeset.changes == %{},
          do: {:ok, tenant}, else: Supavisor.Tenants.update_tenant(tenant, params)
    end

  case result do
    {:ok, _} -> IO.puts("Supavisor tenant configuration applied")
    _ -> raise "Tenant configuration failed"
  end
rescue
  _ ->
    IO.puts(:stderr, "Supavisor tenant initialization failed; check database connectivity and configuration")
    System.halt(1)
catch
  _, _ ->
    IO.puts(:stderr, "Supavisor tenant initialization failed")
    System.halt(1)
end
