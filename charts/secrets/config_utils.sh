#!/bin/bash
# Utility functions for handling YAML configuration

# Dependency check
if ! command -v yq &> /dev/null; then
  echo "yq is not installed. Please install it to use YAML configuration."
  echo "Visit: https://github.com/mikefarah/yq#install"
  exit 1
fi

CONFIG_FILE="config.yaml"
DEFAULT_CONFIG="config.yaml.example"

# Initialize config file if it doesn't exist
init_config() {
  if [ ! -f "$CONFIG_FILE" ]; then
    if [ -f "$DEFAULT_CONFIG" ]; then
      cp "$DEFAULT_CONFIG" "$CONFIG_FILE"
      echo "Created config.yaml from example template"
    else
      echo "Error: config.yaml.example not found"
      exit 1
    fi
  fi
}

# Get a value from the config file
# Usage: get_config_value "path.to.value" "default_value"
get_config_value() {
  local path="$1"
  local default="$2"
  
  # Initialize config if needed
  init_config
  
  # Get the value from the config file
  value=$(yq eval ".$path" "$CONFIG_FILE")
  
  # If the value is null, tilde (~), or empty, return the default
  if [ "$value" = "null" ] || [ "$value" = "" ] || [ "$value" = "~" ]; then
    echo "$default"
  else
    echo "$value"
  fi
}

# Check if a value in the config is set to null/tilde
# Usage: is_null_value "path.to.value"
is_null_value() {
  local path="$1"
  
  # Initialize config if needed
  init_config
  
  # Get the value from the config file
  value=$(yq eval ".$path" "$CONFIG_FILE")
  
  # Check if the value is null or tilde
  if [ "$value" = "null" ] || [ "$value" = "~" ]; then
    return 0  # True, it's null
  else
    return 1  # False, it's not null
  fi
}

# Set a value in the config file
# Usage: set_config_value "path.to.value" "new_value"
set_config_value() {
  local path="$1"
  local value="$2"
  
  # Initialize config if needed
  init_config
  
  # Set the value in the config file
  yq eval ".$path = \"$value\"" -i "$CONFIG_FILE"
}

# Prompt for a value with default from config file
# Usage: prompt_with_config "prompt" "config.path" "default_if_not_in_config" "secure" "var_name" ["can_generate"]
prompt_with_config() {
  local prompt="$1"
  local config_path="$2"
  local default_value="$3"
  local secure="$4"
  local var_name="$5"
  local can_generate="${6:-false}"
  
  # Get the value from config or use default
  local config_value=$(get_config_value "$config_path" "$default_value")
  local is_null=$(is_null_value "$config_path" && echo "true" || echo "false")
  local has_existing_value="false"
  
  # Check if we have a real existing value (not null, not empty, not default)
  if [ "$is_null" = "false" ] && [ -n "$config_value" ] && [ "$config_value" != "$default_value" ]; then
    has_existing_value="true"
  fi
  
  # Set displayed prompt based on whether the value can be generated
  local display_prompt="$prompt"
  if [ "$is_null" = "true" ] && [ "$can_generate" = "true" ]; then
    display_prompt="$prompt (leave empty to generate)"
    config_value="(generate)"
  elif [ "$has_existing_value" = "true" ]; then
    display_prompt="$prompt (leave empty to keep existing value)"
  fi
  
  # Prompt for input
  if [ "$secure" = "true" ]; then
    # For secure input, show stars if there's an existing value
    if [ "$is_null" = "true" ] && [ "$can_generate" = "true" ]; then
      echo "$display_prompt: "
    elif [ "$has_existing_value" = "true" ]; then
      # Show stars for existing password
      local stars=$(printf '%*s' ${#config_value} | tr ' ' '*')
      echo "$display_prompt [$stars]: "
    else
      echo "$prompt: "
    fi
    read -s $var_name
    echo
  else
    # For non-secure input, show the actual value
    read -p "$display_prompt [$config_value]: " $var_name
  fi
  
  # If input is empty and we can generate, generate a value
  local input_value=$(eval echo \$$var_name)
  if [ -z "$input_value" ] && [ "$is_null" = "true" ] && [ "$can_generate" = "true" ]; then
    # Return empty to signal the caller to generate a value
    eval "$var_name=''"
    return 0
  fi
  
  # If input is empty, use the config value (unless it's the generate marker)
  if [ -z "$input_value" ]; then
    if [ "$config_value" = "(generate)" ]; then
      eval "$var_name=''"
    else
      eval "$var_name=$config_value"
      if [ "$has_existing_value" = "true" ] && [ "$secure" = "true" ]; then
        echo "Keeping existing value"
      fi
    fi
  fi
  
  # Save the value back to config if it's not empty
  local final_value=$(eval echo \$$var_name)
  if [ -n "$final_value" ]; then
    set_config_value "$config_path" "$final_value"
  fi
} 