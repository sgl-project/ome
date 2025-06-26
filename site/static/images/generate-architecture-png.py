#!/usr/bin/env python3
import subprocess
import os
import sys

# Check if required tools are installed
try:
    # Use Chrome/Chromium headless to convert HTML to PNG
    html_file = "ome-architecture.html"
    output_file = "ome-architecture.png"
    
    # Try different Chrome/Chromium paths
    chrome_paths = [
        "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
        "/Applications/Chromium.app/Contents/MacOS/Chromium",
        "google-chrome",
        "chromium",
        "chromium-browser"
    ]
    
    chrome_cmd = None
    for path in chrome_paths:
        try:
            subprocess.run([path, "--version"], capture_output=True, check=True)
            chrome_cmd = path
            break
        except:
            continue
    
    if chrome_cmd:
        print(f"Using {chrome_cmd} to generate PNG...")
        cmd = [
            chrome_cmd,
            "--headless",
            "--disable-gpu",
            "--screenshot=" + output_file,
            "--window-size=1600,1200",
            "--default-background-color=0",
            "file://" + os.path.abspath(html_file)
        ]
        subprocess.run(cmd, check=True)
        print(f"Successfully generated {output_file}")
    else:
        print("Chrome/Chromium not found. Trying alternative method...")
        
        # Alternative: Try using wkhtmltoimage if available
        try:
            subprocess.run(["wkhtmltoimage", "--version"], capture_output=True, check=True)
            cmd = [
                "wkhtmltoimage",
                "--width", "1600",
                "--height", "1200",
                "--quality", "100",
                html_file,
                output_file
            ]
            subprocess.run(cmd, check=True)
            print(f"Successfully generated {output_file} using wkhtmltoimage")
        except:
            print("wkhtmltoimage not found either.")
            print("\nTo generate the PNG, you can:")
            print("1. Open ome-architecture.html in a browser")
            print("2. Take a screenshot or use browser's 'Save as Image' feature")
            print("3. Or install Chrome/Chromium or wkhtmltoimage")
            sys.exit(1)

except Exception as e:
    print(f"Error: {e}")
    sys.exit(1)