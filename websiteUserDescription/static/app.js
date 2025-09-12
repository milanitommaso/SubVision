// Global variables
let currentUserId = '';

// DOM elements
const userIdInput = document.getElementById('userIdInput');
const loadUserBtn = document.getElementById('loadUserBtn');
const userDataSection = document.getElementById('userDataSection');
const descriptionSection = document.getElementById('descriptionSection');
const resultSection = document.getElementById('resultSection');
const descriptionForm = document.getElementById('descriptionForm');
const descriptionInput = document.getElementById('descriptionInput');
const submitBtn = document.getElementById('submitBtn');
const submitText = document.getElementById('submitText');
const loadingSpinner = document.getElementById('loadingSpinner');
const charCount = document.getElementById('charCount');

// Initialize the application
document.addEventListener('DOMContentLoaded', function() {
    // Add event listeners
    userIdInput.addEventListener('keypress', function(e) {
        if (e.key === 'Enter') {
            loadUserData();
        }
    });

    descriptionInput.addEventListener('input', updateCharCount);
    descriptionForm.addEventListener('submit', handleFormSubmit);

    // Focus on user ID input
    userIdInput.focus();
});

// Load user data from the server
async function loadUserData() {
    const userId = userIdInput.value.trim();
    
    if (!userId) {
        showError('Please enter a valid User ID');
        return;
    }

    // Show loading state
    loadUserBtn.disabled = true;
    loadUserBtn.innerHTML = '<div class="spinner"></div> Loading...';

    try {
        const response = await fetch(`/api/user-data?userId=${encodeURIComponent(userId)}`);
        
        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const userData = await response.json();
        
        // Store current user ID
        currentUserId = userId;
        
        // Display user data
        displayUserData(userData);
        
        // Show the form sections
        showSections();
        
    } catch (error) {
        console.error('Error loading user data:', error);
        showError('Failed to load user data. Please try again.');
    } finally {
        // Reset button state
        loadUserBtn.disabled = false;
        loadUserBtn.innerHTML = 'Load My Data';
    }
}

// Display user data in the UI
function displayUserData(userData) {
    document.getElementById('displayUserId').textContent = userData.userId;
    document.getElementById('currentDescription').textContent = userData.description;
    document.getElementById('lastUpdated').textContent = userData.lastUpdated;
}

// Show the user data and form sections
function showSections() {
    userDataSection.style.display = 'block';
    descriptionSection.style.display = 'block';
    
    // Scroll to the form section
    descriptionSection.scrollIntoView({ behavior: 'smooth' });
}

// Handle form submission
async function handleFormSubmit(e) {
    e.preventDefault();
    
    const description = descriptionInput.value.trim();
    
    if (!description) {
        showError('Please enter a description');
        return;
    }

    if (description.length < 10) {
        showError('Description must be at least 10 characters long');
        return;
    }

    // Show loading state
    setSubmitButtonLoading(true);
    hideResult();

    try {
        const response = await fetch('/api/submit-description', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                userId: currentUserId,
                description: description
            })
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const result = await response.json();
        
        // Show result
        showResult(result);
        
        // If successful, reload user data to show updated information
        if (result.success && result.valid) {
            setTimeout(() => {
                loadUserDataSilently();
            }, 1000);
        }
        
    } catch (error) {
        console.error('Error submitting description:', error);
        showError('Failed to submit description. Please try again.');
    } finally {
        // Reset button state
        setSubmitButtonLoading(false);
    }
}

// Load user data silently (without showing loading states)
async function loadUserDataSilently() {
    try {
        const response = await fetch(`/api/user-data?userId=${encodeURIComponent(currentUserId)}`);
        
        if (response.ok) {
            const userData = await response.json();
            displayUserData(userData);
        }
    } catch (error) {
        console.error('Error reloading user data:', error);
    }
}

// Set submit button loading state
function setSubmitButtonLoading(isLoading) {
    submitBtn.disabled = isLoading;
    
    if (isLoading) {
        submitText.textContent = 'Processing...';
        loadingSpinner.style.display = 'block';
    } else {
        submitText.textContent = 'Submit Description';
        loadingSpinner.style.display = 'none';
    }
}

// Update character count
function updateCharCount() {
    const count = descriptionInput.value.length;
    charCount.textContent = count;
    
    // Change color based on minimum requirement
    if (count < 10) {
        charCount.style.color = '#e74c3c';
    } else {
        charCount.style.color = '#27ae60';
    }
}

// Show result message
function showResult(result) {
    const resultMessage = document.getElementById('resultMessage');
    
    // Clear previous classes
    resultMessage.className = 'result-message';
    
    if (result.success && result.valid) {
        resultMessage.classList.add('success');
        resultMessage.innerHTML = `
            <h3>✅ Success!</h3>
            <p>${result.message}</p>
        `;
        
        // Clear the form
        descriptionInput.value = '';
        updateCharCount();
        
    } else if (!result.valid) {
        resultMessage.classList.add('error');
        resultMessage.innerHTML = `
            <h3>❌ Description Rejected</h3>
            <p>${result.message}</p>
            <p><small>Please modify your description and try again.</small></p>
        `;
        
    } else {
        resultMessage.classList.add('error');
        resultMessage.innerHTML = `
            <h3>❌ Error</h3>
            <p>${result.message || 'An unexpected error occurred.'}</p>
        `;
    }
    
    // Show result section
    resultSection.style.display = 'block';
    
    // Scroll to result
    resultSection.scrollIntoView({ behavior: 'smooth' });
}

// Show error message
function showError(message) {
    const resultMessage = document.getElementById('resultMessage');
    
    resultMessage.className = 'result-message error';
    resultMessage.innerHTML = `
        <h3>❌ Error</h3>
        <p>${message}</p>
    `;
    
    resultSection.style.display = 'block';
    resultSection.scrollIntoView({ behavior: 'smooth' });
}

// Hide result section
function hideResult() {
    resultSection.style.display = 'none';
}

// Utility function to show info message
function showInfo(message) {
    const resultMessage = document.getElementById('resultMessage');
    
    resultMessage.className = 'result-message info';
    resultMessage.innerHTML = `
        <h3>ℹ️ Information</h3>
        <p>${message}</p>
    `;
    
    resultSection.style.display = 'block';
    resultSection.scrollIntoView({ behavior: 'smooth' });
}

// Handle Enter key in description textarea (Ctrl+Enter to submit)
descriptionInput.addEventListener('keydown', function(e) {
    if (e.key === 'Enter' && e.ctrlKey) {
        e.preventDefault();
        handleFormSubmit(e);
    }
});

// Add some visual feedback for form validation
descriptionInput.addEventListener('blur', function() {
    const description = this.value.trim();
    
    if (description.length > 0 && description.length < 10) {
        this.style.borderColor = '#e74c3c';
    } else if (description.length >= 10) {
        this.style.borderColor = '#27ae60';
    } else {
        this.style.borderColor = '#e1e8ed';
    }
});

descriptionInput.addEventListener('focus', function() {
    this.style.borderColor = '#4facfe';
});
