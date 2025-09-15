// DOM elements
const loginSection = document.getElementById('loginSection');
const userDataSection = document.getElementById('userDataSection');
const descriptionFormSection = document.getElementById('descriptionFormSection');
const resultSection = document.getElementById('resultSection');
const descriptionForm = document.getElementById('descriptionForm');
const descriptionInput = document.getElementById('descriptionInput');
const submitBtn = document.getElementById('submitBtn');
const submitText = document.getElementById('submitText');
const loadingSpinner = document.getElementById('loadingSpinner');
const charCount = document.getElementById('charCount');
const userButtonContainer = document.getElementById('userButtonContainer');


window.addEventListener('load', async function () {
    await Clerk.load()
    if (Clerk.isSignedIn) {
        // Show user button in header
        const userButtonDiv = document.getElementById('userButton');
        Clerk.mountUserButton(userButtonDiv);
        userButtonContainer.style.display = 'flex';

        // Hide login section and show user sections
        loginSection.style.display = 'none';
        loadUserData();
        showLoggedInSections();
    } else {
        // Hide user button and show login section
        userButtonContainer.style.display = 'none';
        loginSection.style.display = 'block';
        
        const signInDiv = document.getElementById('sign-in');
        Clerk.mountSignIn(signInDiv);
    }
})

document.addEventListener('DOMContentLoaded', function() {
    descriptionInput.addEventListener('input', updateCharCount);
    descriptionForm.addEventListener('submit', handleFormSubmit);
});

// Load user data from the server
async function loadUserData() {
    try {
        const response = await fetch('/api/user-data', {
            headers: {
                Authorization: `Bearer ${await Clerk.session.getToken()}`,
            },
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const userData = await response.json();
        // Display user data
        displayUserData(userData);
    } catch (error) {
        console.error('Error loading user data:', error);
        showError('Failed to load user data. Please try again.');
    }
}

// Display user data in the UI
function displayUserData(userData) {
    document.getElementById('displayUsername').textContent = userData.username;
    document.getElementById('currentDescription').textContent = userData.description;
    
    try {
        const formattedDate = new Date(userData.lastUpdated).toLocaleString("IT-it");
        document.getElementById('lastUpdated').textContent = formattedDate;
        if (formattedDate === "Invalid Date") {
            document.getElementById('lastUpdated').textContent = userData.lastUpdated;
        }
    } catch (e) {
        console.error('Error formatting date:', e);
        document.getElementById('lastUpdated').textContent = userData.lastUpdated;
    }
}

// Show username, user description and form sections
function showLoggedInSections() {
    userDataSection.style.display = 'block';
    descriptionFormSection.style.display = 'block';
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
                Authorization: `Bearer ${await Clerk.session.getToken()}`,
            },
            body: JSON.stringify({
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
                loadUserData();
            }, 500);
        }
        
    } catch (error) {
        console.error('Error submitting description:', error);
        showError('Failed to submit description. Please try again.');
    } finally {
        // Reset button state
        setSubmitButtonLoading(false);
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
        charCount.style.color = '#ef4444';
    } else {
        charCount.style.color = '#10b981';
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

// Add some visual feedback for form validation
descriptionInput.addEventListener('blur', function() {
    const description = this.value.trim();
    
    if (description.length > 0 && description.length < 10) {
        this.style.borderColor = '#ef4444';
    } else if (description.length >= 10) {
        this.style.borderColor = '#10b981';
    } else {
        this.style.borderColor = '#2a2a32';
    }
});

descriptionInput.addEventListener('focus', function() {
    this.style.borderColor = '#6366f1';
});
