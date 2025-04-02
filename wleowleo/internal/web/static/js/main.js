document.addEventListener("DOMContentLoaded", function () {
  // Initialize Feather icons
  if (window.feather) {
    feather.replace();
  }

  // Initialize tooltips for form inputs
  const inputContainers = document.querySelectorAll(".relative input");
  inputContainers.forEach((input) => {
    const tooltip = input.parentElement.querySelector(".tooltip");
    if (tooltip) {
      input.addEventListener("mouseenter", () => {
        tooltip.classList.remove("hidden");
      });
      input.addEventListener("mouseleave", () => {
        tooltip.classList.add("hidden");
      });
    }
  });

  // Connect to WebSocket
  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const wsUrl = `${protocol}//${window.location.host}/ws`;
  const socket = new WebSocket(wsUrl);

  // DOM Elements
  const startButton = document.getElementById("start-button");
  const stopButton = document.getElementById("stop-button");
  const scraperLogs = document.getElementById("scraper-logs");
  const downloaderLogs = document.getElementById("downloader-logs");
  const totalPageScraped = document.getElementById("total-page-scraped");
  const totalScrapedLink = document.getElementById("total-scraped-link");
  const videoDownloaded = document.getElementById("video-downloaded");

  // Track scraping process state - retrieve from localStorage if available
  let isProcessRunning = localStorage.getItem("isProcessRunning") === "true";

  // Set initial UI state based on saved state
  setProcessRunning(isProcessRunning);

  // WebSocket event handlers
  socket.onopen = function (e) {
    console.log("WebSocket connection established");
    showToast("Connected to server", "success");
    document.querySelectorAll("span").forEach((span) => {
      if (span.textContent === "Live") {
        span.classList.add("bg-success-color");
      }
    });
  };

  // WebSocket message event
  socket.onmessage = function (event) {
    const data = JSON.parse(event.data);
    console.log("Message from server:", data);
    switch (data.type) {
      case "scraper_log":
        addScraperLog(
          data.data.title,
          data.data.m3u8 || data.error,
          data.status,
        );
        updateStats(data.stats);
        break;
      case "downloader_log":
        addDownloaderLog(data.message, data.progress || 0, data.total || 1);
        break;
      case "stats":
        updateStats(data.stats);
        break;
      case "status":
        showToast(data.message, data.status || "info");
        if (data.message.includes("started")) {
          setProcessRunning(true);
        } else if (
          data.message.includes("stopped") ||
          data.message.includes("completed")
        ) {
          setProcessRunning(false);
        }
        break;
    }
  };

  // WebSocket close event
  socket.onclose = function (event) {
    if (event.wasClean) {
      console.log(
        `Connection closed cleanly, code=${event.code} reason=${event.reason}`,
      );
      showToast(`Connection closed: ${event.reason}`, "info");
    } else {
      console.log("Connection died");
      showToast("Connection to server lost. Please refresh the page.", "error");
    }

    // Update connection indicators
    document.querySelectorAll("span").forEach((span) => {
      if (span.textContent === "Live") {
        span.classList.remove("bg-success-color");
        span.classList.add("bg-error-color");
      }
    });
  };

  // Error handling
  socket.onerror = function (error) {
    console.error(`WebSocket error: ${error.message}`);
    showToast(`WebSocket error: ${error.message}`, "error");
  };

  // Function to set the process running state and update UI accordingly
  function setProcessRunning(running) {
    isProcessRunning = running;

    // Store in localStorage to persist across refreshes
    localStorage.setItem("isProcessRunning", running);

    if (running) {
      // Process is running - disable start, enable stop
      startButton.disabled = true;
      startButton.classList.add("opacity-50", "cursor-not-allowed");
      startButton.innerHTML = `
        <i data-feather="play" class="h-4 w-4"></i>
        <span>Start Process</span>
      `;

      stopButton.disabled = false;
      stopButton.classList.remove("opacity-50", "cursor-not-allowed");
      stopButton.classList.add("bg-error");
      stopButton.innerHTML = `
        <i data-feather="square" class="h-4 w-4"></i>
        <span>Stop Process</span>
      `;
    } else {
      // Process is not running - enable start, disable stop
      startButton.disabled = false;
      startButton.classList.remove("opacity-50", "cursor-not-allowed");
      startButton.innerHTML = `
        <i data-feather="play" class="h-4 w-4"></i>
        <span>Start Process</span>
      `;

      stopButton.disabled = true;
      stopButton.classList.add("opacity-50", "cursor-not-allowed");
      stopButton.classList.remove("bg-error");
      stopButton.innerHTML = `
        <i data-feather="square" class="h-4 w-4"></i>
        <span>Stop Process</span>
      `;
    }

    // Refresh feather icons
    if (window.feather) {
      feather.replace();
    }
  }

  // Event listeners
  startButton.addEventListener("click", function () {
    if (isProcessRunning) return; // Prevent multiple clicks

    // Show loading state
    startButton.disabled = true;
    startButton.classList.add("opacity-50", "cursor-not-allowed");
    startButton.innerHTML = `
            <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
            <span>Starting...</span>
        `;

    function Start() {
      const config = {
        from_pages: parseInt(document.getElementById("from-page").value),
        to_pages: parseInt(document.getElementById("to-page").value),
        concurrent_download: parseInt(
          document.getElementById("concurrent-download").value,
        ),
      };

      // Save form values to localStorage for persistence
      localStorage.setItem("from_pages", config.from_pages);
      localStorage.setItem("to_pages", config.to_pages);
      localStorage.setItem("concurrent_download", config.concurrent_download);

      fetch("/api/start", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(config),
      })
        .then((response) => {
          if (!response.ok) {
            throw new Error("Failed to start scraping");
          }
          return response.json();
        })
        .then((data) => {
          console.log("Scraping started:", data);
          showToast("Scraping process started successfully", "success");
          setProcessRunning(true);
        })
        .catch((error) => {
          console.error("Error starting scraping:", error);
          showToast(`Failed to start scraping: ${error.message}`, "error");

          // Reset button state
          setProcessRunning(false);
          startButton.innerHTML = `
                    <i data-feather="play"></i>
                    <span>Start Process</span>
                `;
          if (window.feather) {
            feather.replace();
          }
        });
    }
    Start();
  });

  // Add event listener for stop button
  stopButton.addEventListener("click", function () {
    if (!isProcessRunning) return; // Prevent clicks if not running

    // Show loading state
    stopButton.disabled = true;
    stopButton.classList.add("opacity-50", "cursor-not-allowed");
    stopButton.innerHTML = `
            <div class="animate-spin h-4 w-4 border-2 border-white border-t-transparent rounded-full"></div>
            <span>Stopping...</span>
        `;

    fetch("/api/stop", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to stop scraping");
        }
        return response.json();
      })
      .then((data) => {
        console.log("Scraping stopped:", data);
        showToast("Scraping process stopped successfully", "info");

        // We'll wait for the server to confirm the process is stopped
        // The WebSocket status message handler will update the UI

        // If we don't get a WebSocket confirmation within 3 seconds, update UI anyway
        setTimeout(() => {
          if (isProcessRunning) {
            setProcessRunning(false);
          }
        }, 3000);
      })
      .catch((error) => {
        console.error("Error stopping scraping:", error);
        showToast(`Failed to stop scraping: ${error.message}`, "error");

        // Reset stop button UI
        if (isProcessRunning) {
          stopButton.disabled = false;
          stopButton.classList.remove("opacity-50", "cursor-not-allowed");
          stopButton.innerHTML = `
                <i data-feather="square"></i>
                <span>Stop Process</span>
            `;
          if (window.feather) {
            feather.replace();
          }
        }
      });
  });

  // Load saved form values from localStorage
  function loadSavedFormValues() {
    const fromPages = localStorage.getItem("from_pages");
    const toPages = localStorage.getItem("to_pages");
    const concurrentDownload = localStorage.getItem("concurrent_download");

    if (fromPages) {
      document.getElementById("from-page").value = fromPages;
    }

    if (toPages) {
      document.getElementById("to-page").value = toPages;
    }

    if (concurrentDownload) {
      document.getElementById("concurrent-download").value = concurrentDownload;
    }
  }

  // Load saved form values when the page loads
  loadSavedFormValues();

  // Update stats
  function updateStats(data) {
    // Animate the counter
    animateCounter(totalPageScraped, data.total_page_scraped);
    animateCounter(totalScrapedLink, data.total_scraped_link);
    animateCounter(videoDownloaded, data.video_downloaded);
  }

  // Animate counter
  function animateCounter(element, newValue) {
    const currentValue = parseInt(element.textContent) || 0;
    if (currentValue === newValue) return;

    const step = Math.max(
      1,
      Math.floor(Math.abs(newValue - currentValue) / 20),
    );
    let current = currentValue;

    const interval = setInterval(() => {
      if (current < newValue) {
        current = Math.min(current + step, newValue);
      } else if (current > newValue) {
        current = Math.max(current - step, newValue);
      }

      element.textContent = current;

      if (current === newValue) {
        clearInterval(interval);
      }
    }, 30);
  }

  // Logging functions
  function addScraperLog(title, message, status) {
    // Check if a log with the same title already exists
    const existingItem = Array.from(scraperLogs.children).find((item) => {
      const titleElement = item.querySelector("p.font-medium");
      return titleElement && titleElement.textContent === title;
    });

    const timestamp = new Date().toLocaleTimeString();
    let borderColor = "border-l-success";

    if (status === "error") {
      borderColor = "border-l-error"; // Red color for other errors
    }

    if (status === "warm") {
      borderColor = "border-l-warning"; // Orange color for retry attempts errors
    }

    if (existingItem) {
      // Update existing log item
      const messageElement = existingItem.querySelector(".text-sm.mt-1");
      const timestampElement = existingItem.querySelector(
        ".text-xs.opacity-70",
      );

      // Update message and timestamp
      if (messageElement) messageElement.textContent = message;
      if (timestampElement) timestampElement.textContent = timestamp;

      // Update border color based on status
      existingItem.className = existingItem.className.replace(
        /border-l-(error|success|warning)/,
        borderColor,
      );

      // Highlight the updated item with a brief animation
      existingItem.style.transition = "background-color 0.3s ease";
      existingItem.style.backgroundColor = "rgba(255, 255, 255, 0.1)";
      setTimeout(() => {
        existingItem.style.backgroundColor = "";
      }, 300);

      return;
    }

    // Create new log item if it doesn't exist
    const logItem = document.createElement("div");
    logItem.className = `p-3 mb-2 rounded-md bg-bg-dark border-l-4 ${borderColor} transition-transform duration-200 hover:translate-x-1 w-full`;

    logItem.innerHTML = `
            <div class="flex justify-between items-start">
                <p class="font-medium">${title}</p>
                <span class="text-xs opacity-70">${timestamp}</span>
            </div>
            <p class="text-sm mt-1 truncate">${message}</p>
        `;

    // Add with animation at the top
    logItem.style.opacity = "0";
    logItem.style.transform = "translateY(-10px)";
    scraperLogs.prepend(logItem);

    // Trigger animation
    setTimeout(() => {
      logItem.style.transition = "opacity 0.3s ease, transform 0.3s ease";
      logItem.style.opacity = "1";
      logItem.style.transform = "translateY(0)";
    }, 10);

    // Keep scroll at top
    setTimeout(() => {
      scraperLogs.scrollTop = 0;
    }, 50);
  }

  // Add downloader log
  function addDownloaderLog(message, progress, total) {
    // Find if we already have a log item for this video
    const existingItem = Array.from(downloaderLogs.children).find((item) => {
      const title = item.querySelector("p.font-medium");
      return title && title.textContent === message;
    });

    const percent = Math.round((progress / total) * 100);

    if (existingItem) {
      // Update existing log item
      const progressBar = existingItem.querySelector(".bg-success");
      const percentText = existingItem.querySelector(".text-xs.opacity-70");
      const progressText = existingItem.querySelector(".text-xs.text-right");

      if (progressBar) progressBar.style.width = `${percent}%`;
      if (percentText) percentText.textContent = `${percent}%`;
      if (progressText) progressText.textContent = `${progress}/${total}`;

      // If complete (100%), highlight the item
      if (percent === 100) {
        existingItem.classList.add("border-l-success");

        // Show a toast for completion
        showToast(`Download completed: ${message}`, "success");
      }

      return;
    }

    // Create new log item if it doesn't exist
    const logItem = document.createElement("div");
    const borderColor =
      percent === 100 ? "border-l-success" : "border-l-transparent";
    logItem.className = `p-3 mb-2 rounded-md bg-bg-dark border-l-4 ${borderColor} transition-transform duration-200 hover:translate-x-1 w-full`;

    logItem.innerHTML = `
            <div class="flex justify-between items-start">
                <p class="font-medium">${message}</p>
                <span class="text-xs opacity-70">${percent}%</span>
            </div>
            <div class="h-2.5 bg-border-color rounded-md overflow-hidden mt-2">
                <div class="h-full bg-success rounded-md transition-all duration-300" style="width: ${percent}%"></div>
            </div>
            <p class="text-xs text-right mt-1">${progress}/${total}</p>
        `;

    // Add with animation at the top
    logItem.style.opacity = "0";
    logItem.style.transform = "translateY(-10px)";
    downloaderLogs.prepend(logItem);

    // Trigger animation
    setTimeout(() => {
      logItem.style.transition = "opacity 0.3s ease, transform 0.3s ease";
      logItem.style.opacity = "1";
      logItem.style.transform = "translateY(0)";
    }, 10);

    // Keep scroll at top
    setTimeout(() => {
      downloaderLogs.scrollTop = 0;
    }, 50);

    // Show toast when download completes
    if (percent === 100) {
      showToast(`Download completed: ${message}`, "success");
    }
  }

  // Toast notification system
  function showToast(message, type = "info") {
    // Create toast container if it doesn't exist
    let toastContainer = document.querySelector("#toast-container");
    if (!toastContainer) {
      toastContainer = document.createElement("div");
      toastContainer.id = "toast-container";
      toastContainer.className =
        "fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-md";
      document.body.appendChild(toastContainer);
    }

    // Create toast element
    const toast = document.createElement("div");

    // Set background and border color based on type
    let bgColor = "bg-panel-bg";
    let borderColor = "border-primary";
    let iconColor = "text-primary";

    if (type === "success") {
      borderColor = "border-success";
      iconColor = "text-success";
    } else if (type === "error") {
      borderColor = "border-error";
      iconColor = "text-error";
    }

    toast.className = `flex items-center p-4 mb-2 rounded-lg shadow-custom ${bgColor} border-l-4 ${borderColor} animate-slideIn`;

    // Get appropriate icon based on type
    let iconName = "info";
    if (type === "success") iconName = "check-circle";
    if (type === "error") iconName = "alert-circle";

    toast.innerHTML = `
            <div class="flex-shrink-0 mr-3 ${iconColor}">
                <i data-feather="${iconName}"></i>
            </div>
            <div class="flex-grow text-sm text-text-light">${message}</div>
            <button class="ml-3 flex-shrink-0 text-border-color hover:text-text-light transition-colors duration-200">
                <i data-feather="x"></i>
            </button>
        `;

    // Add to container
    toastContainer.appendChild(toast);

    // Initialize Feather icons
    if (window.feather) {
      feather.replace();
    }

    // Add close functionality
    const closeBtn = toast.querySelector("button");
    closeBtn.addEventListener("click", () => {
      toast.classList.remove("animate-slideIn");
      toast.classList.add("animate-slideOut");
      setTimeout(() => {
        toast.remove();
        // Remove container if empty
        if (toastContainer.children.length === 0) {
          toastContainer.remove();
        }
      }, 300);
    });

    // Auto remove after 5 seconds
    setTimeout(() => {
      if (toast.parentNode) {
        toast.classList.remove("animate-slideIn");
        toast.classList.add("animate-slideOut");
        setTimeout(() => {
          if (toast.parentNode) {
            toast.remove();
            // Remove container if empty
            if (toastContainer.children.length === 0) {
              toastContainer.remove();
            }
          }
        }, 300);
      }
    }, 5000);
  }

  // Add some initial dummy data for demonstration
  setTimeout(() => {
    // Add extra information if process is running
    if (isProcessRunning) {
      showToast("Scraping process is currently running", "info");
    }
  }, 500); // Slight delay for better UX
});
