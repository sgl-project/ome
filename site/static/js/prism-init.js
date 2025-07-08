// Initialize Prism.js after page load
document.addEventListener('DOMContentLoaded', function() {
  // Check if Prism is loaded
  if (typeof Prism !== 'undefined') {
    // Re-run Prism highlighting in case any code blocks were dynamically added
    Prism.highlightAll();
  }
});