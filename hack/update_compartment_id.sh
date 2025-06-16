#!/bin/bash

# Script to move compartmentID from spec to metadata.labels in all model resource YAML files
cd /Users/simolin/golang/src/bitbucket.oci.oraclecorp.com/genaicore/ome
MODELS_DIR="./config/models"

find $MODELS_DIR -name "*.yaml" -o -name "*.yml" | while read -r file; do
  echo "Processing $file..."
  
  # Check if the file contains compartmentID under spec
  if grep -q "compartmentID:" "$file"; then
    # Extract the compartment ID value (with proper quoting preserved)
    COMPARTMENT_ID=$(grep -o "compartmentID: .*$" "$file" | sed 's/compartmentID: //')
    
    # Remove any trailing whitespace but preserve quotes if they exist
    COMPARTMENT_ID=$(echo "$COMPARTMENT_ID" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//')
    
    # Create a temporary file
    temp_file=$(mktemp)
    
    # Process the file using awk for more precise handling
    awk -v compartment_id="$COMPARTMENT_ID" '
    BEGIN { 
      in_metadata = 0; 
      metadata_section_ended = 0;
      labels_found = 0; 
      removed_compartment_id = 0;
      added_label = 0;
    }
    
    # When we find metadata section, mark it
    /^metadata:/ { 
      in_metadata = 1; 
      print; 
      next; 
    }
    
    # Check for the end of metadata section
    in_metadata == 1 && /^[a-z]/ { 
      # If we have reached the end of metadata section without finding labels, add them now
      if (!labels_found) {
        print "  labels:"
        print "    ome.io/oci-compartmentid: " compartment_id
        added_label = 1
      }
      in_metadata = 0
      metadata_section_ended = 1
    }
    
    # If we find labels in metadata section
    in_metadata == 1 && /^  labels:/ { 
      print
      print "    ome.io/oci-compartmentid: " compartment_id
      labels_found = 1
      added_label = 1
      next
    }
    
    # Skip the compartmentID line under spec
    /^[[:space:]]*compartmentID:/ { 
      removed_compartment_id = 1
      next 
    }
    
    # Print all other lines
    { print }
    
    END {
      # If we never found the labels section and never added it, add it at the end of metadata
      if (!added_label && !metadata_section_ended && in_metadata) {
        print "  labels:"
        print "    ome.io/oci-compartmentid: " compartment_id
      }
    }' "$file" > "$temp_file"
    
    # Check if processing was successful
    if [ $? -eq 0 ] && grep -q "ome.io/oci-compartmentid:" "$temp_file"; then
      # Make backup of original file
      cp "$file" "${file}.bak"
      # Replace the original file with the modified one
      mv "$temp_file" "$file"
      echo "Updated $file successfully"
    else
      echo "WARNING: Could not modify $file correctly - check file structure"
      cat "$temp_file" > "${file}.attempted"
      rm "$temp_file"
    fi
  else
    echo "No compartmentID found in $file, skipping"
  fi
done

echo "Done processing all files"
