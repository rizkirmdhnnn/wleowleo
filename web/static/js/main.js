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
  const scraperLogs = document.getElementById("scraper-logs");
  const downloaderLogs = document.getElementById("downloader-logs");
  const totalPageScraped = document.getElementById("total-page-scraped");
  const totalScrapedLink = document.getElementById("total-scraped-link");
  const videoDownloaded = document.getElementById("video-downloaded");
  const configForm = document.getElementById("config-form");

  // WebSocket event handlers
  socket.onopen = function (e) {
    console.log("WebSocket connection established");
    showToast("Connected to server", "success");
    // Request initial stats
    fetchStats();

    // Add connected indicator
    document.querySelectorAll(".panel span").forEach((span) => {
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
        addDownloaderLog(data.message, data.progress, data.total);
        break;
      case "stats":
        updateStats(data);
        break;
      case "status":
        // Handle status messages (like "Scraping started")
        console.log(data.message);
        showToast(data.message, data.status || "info");
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
    document.querySelectorAll(".panel span").forEach((span) => {
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

  // Event listeners
  startButton.addEventListener("click", function () {
    // Show loading state
    startButton.disabled = true;
    startButton.innerHTML = `
            <div class="loading-spinner"></div>
            <span>Starting...</span>
        `;

    // First update configuration
    updateConfig()
      .then(() => {
        // Then start scraping
        return fetch("/api/start", {
          method: "POST",
        });
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

        // Reset button state but with different text
        startButton.disabled = false;
        startButton.innerHTML = `
                    <i data-feather="refresh-cw"></i>
                    <span>Restart Process</span>
                `;
        if (window.feather) {
          feather.replace();
        }
      })
      .catch((error) => {
        console.error("Error starting scraping:", error);
        showToast(`Failed to start scraping: ${error.message}`, "error");

        // Reset button state
        startButton.disabled = false;
        startButton.innerHTML = `
                    <i data-feather="play"></i>
                    <span>Start Process</span>
                `;
        if (window.feather) {
          feather.replace();
        }
      });
  });

  // Functions
  function fetchStats() {
    fetch("/api/stats")
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to fetch stats");
        }
        return response.json();
      })
      .then((data) => {
        updateStats(data);
      })
      .catch((error) => {
        console.error("Error fetching stats:", error);
        showToast(`Failed to fetch statistics: ${error.message}`, "error");
      });
  }

  // Update configuration
  function updateConfig() {
    const config = {
      base_url: document.getElementById("base-url")
        ? document.getElementById("base-url").value
        : "",
      from_pages: parseInt(document.getElementById("from-page").value),
      to_pages: parseInt(document.getElementById("to-page").value),
      limit_concurrent_download: parseInt(
        document.getElementById("concurrent-download").value,
      ),
    };

    // Validate input
    if (config.from_pages > config.to_pages) {
      showToast("From Page must be less than or equal to To Page", "error");
      return Promise.reject(new Error("Invalid page range"));
    }

    if (config.limit_concurrent_download <= 0) {
      showToast("Concurrent Downloads must be greater than 0", "error");
      return Promise.reject(new Error("Invalid concurrent downloads"));
    }

    return fetch("/api/config", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(config),
    })
      .then((response) => {
        if (!response.ok) {
          throw new Error("Failed to update configuration");
        }
        return response.json();
      })
      .then((data) => {
        console.log("Configuration updated:", data);
        showToast("Configuration updated successfully", "success");
        return data;
      })
      .catch((error) => {
        console.error("Error updating configuration:", error);
        showToast(`Failed to update configuration: ${error.message}`, "error");
        throw error;
      });
  }

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
    const logItem = document.createElement("div");
    const borderColor =
      status === "error" ? "border-l-error" : "border-l-success";
    logItem.className = `p-3 mb-2 rounded-md bg-bg-dark border-l-4 ${borderColor} transition-transform duration-200 hover:translate-x-1 w-full`;

    const timestamp = new Date().toLocaleTimeString();

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

    // If error, show toast
    // if (status === 'error') {
    //     showToast(`Error: ${message}`, 'error');
    // }
  }

  // Add downloader log
  function addDownloaderLog(message, progress, total) {
    const logItem = document.createElement("div");
    logItem.className =
      "p-3 mb-2 rounded-md bg-bg-dark border-l-4 border-l-transparent transition-transform duration-200 hover:translate-x-1 w-full";

    const percent = Math.round((progress / total) * 100);

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
    // if (progress === total) {
    //     showToast(`Download completed: ${message}`, 'success');
    // }
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
    // // Add sample scraper logs
    // addScraperLog('Video Title 1', 'm3u8 link not found', 'error');
    // addScraperLog('Video Title 2', 'https://example.com/video1.m3u8', 'success');
    // addScraperLog('Video Title 3', 'https://example.com/video2.m3u8', 'success');
    // addScraperLog('Video Title 4', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 5', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 6', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 7', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 8', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 9', 'https://example.com/video3.m3u8', 'success');
    // addScraperLog('Video Title 10', 'https://example.com/video3.m3u8', 'success');

    // // Add sample downloader logs with different progress
    // addDownloaderLog('Sample Video 1', 5, 50);
    // addDownloaderLog('Sample Video 2', 15, 50);
    // addDownloaderLog('Sample Video 3', 25, 50);
    // addDownloaderLog('Sample Video 4', 35, 50);
    // addDownloaderLog('Sample Video 5', 45, 50);
    // addDownloaderLog('Sample Video 6', 45, 50);
    // addDownloaderLog('Sample Video 7', 45, 50);
    // addDownloaderLog('Sample Video 8', 45, 50);
    // addDownloaderLog('Sample Video 9', 45, 50);
    // addDownloaderLog('Sample Video 10', 50, 50); // Completed download

    // Update stats with initial values (with animation)
    // updateStats({
    //     total_page_scraped: 300,
    //     total_scraped_link: 300,
    //     video_downloaded: 20
    // });

    // Show a welcome toast
    showToast("Welcome to WleoWleo Dashboard", "info");
  }, 500); // Slight delay for better UX
});
