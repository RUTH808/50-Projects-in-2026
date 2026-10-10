1) Pomodoro basic, timer and the usual features.
2) Rock vs Mine prediction used linear regression to find if the object is a rock or a mine from the given data.
3) Doc-Intelligence, useds claude API to give a summary, key points and answer questions based on the .docx or whatever content you've pasted.
4) Prob1 is a QR code decoder, it decodes QR codes assigned in the Images folder and stores in a .csv file.
5) Qrscanner2 is upgraded version of the previous QR scanner/decoder. It can now use CV to read real time QR codes and decode them, since camera not available added a mock camera to test it (also has code incase a real camera is invovled). It also has a dashboard which displays crucial information along with the QR its decoding and stores in a csv.
6) ALPR_Project - Automatic License Plate Recognition, recoginises license plates. Has image processing using OpenCV, plate and character detection using YOLOV11. Uses mock camera, similar to previous project. Needs python version 11 to run it. Used roboflow to train the model, also to run the model use roboflow api.
7) Image Processing Toolkit is a desktop application built using Python, OpenCV, Tkinter, Pillow, and NumPy that allows users to load images from a local data folder, apply various image processing operations through a graphical user interface, preview results instantly, and save processed images. The toolkit supports operations such as grayscale conversion, color conversion, blurring, sharpening, edge detection, thresholding, brightness and contrast adjustment, denoising, image transformations, and more.
Tkinter for GUI. Pillow for image format conversion and rendering inside Tkinter, without it Tkinter cannot render open CV images. Numpy for storing images.
8) OpenCV Image Studio is a desktop image-editing application built using Python, OpenCV, and CustomTkinter. It allows users to load images, apply image-processing operations through a plugin system, create reusable processing pipelines, perform batch processing on entire folders, compare original and edited images side-by-side, view image statistics, and manage editing history through undo/redo functionality.
9) XOR neural network - Build a neural network from scratch using NumPy to learn the XOR logic function.
                        The project demonstrates how hidden layers solve problems that a single perceptron cannot.
                        It implements forward propagation, loss calculation, backpropagation, and gradient descent manually.
                        The goal is to understand the core mechanics of neural networks without using ML frameworks.
10) mnist_digit_classifier - Built a neural network to classify handwritten digits (0–9) using the MNIST dataset.
Preprocessed image data by normalizing pixels and one-hot encoding labels.
Trained a feedforward neural network with ReLU hidden layers and a Softmax output layer.
Evaluated the model on unseen test images and used it to predict handwritten digits.
11) House Price Predictor - Built a neural network to predict house prices using features such as area, bedrooms, bathrooms, and age.
Preprocessed the data by normalizing features and target values for stable training.
Trained a regression model using Dense layers, ReLU activation, MSE loss, and the Adam optimizer.
Used the trained model to estimate prices for new houses and saved the model for future predictions.
12) Fashion-MNIST Clothing Classifier - Built a neural network to classify clothing items from the Fashion-MNIST dataset.
Preprocessed 28×28 grayscale images by normalizing pixel values and one-hot encoding labels.
Trained a multi-class classification model using Dense layers, ReLU activations, Softmax output, and cross-entropy loss.
Evaluated the model on unseen images and predicted one of 10 fashion categories with high accuracy.



note: commands to run node.js 
npm create vite@latest my-project-name -- --template react
cd my-project-name
npm install
npm run dev


## Software engineering portfolio projects

13) [CampusFlow](campusflow/) — Java/Spring Boot and React/TypeScript equipment booking platform with PostgreSQL conflict prevention, role-based approvals, automatic waitlists, maintenance workflows, and audit reporting. [Setup guide](campusflow/README.md) · [Resume description](campusflow/RESUME.md).

14) [HookRelay](hookrelay/) — Go webhook delivery service with transactional ingestion, a PostgreSQL durable queue, leased concurrent workers, signed HTTP delivery, retries, dead letters, replay, and a React/TypeScript console. Includes a Python receiver and failure-recovery demo. [Setup guide](hookrelay/README.md) · [Resume description](hookrelay/RESUME.md).

Both projects include Docker Compose setups and automated checks. They run independently or together using different default ports.
